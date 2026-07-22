//go:build integration

package integrations

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func TestPostgresWebhookFanoutHandlesAutomationWithoutSubscriptionsAndQueuesMatches(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, adminURL, appPassword); err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, adminURL, 1, "webhook-worker-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	app, err := database.Open(ctx, appURL, 2, "webhook-worker-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	workspaceID := mustWebhookWorkerID(t)
	userID := mustWebhookWorkerID(t)
	membershipID := mustWebhookWorkerID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")
	email := "webhook-" + suffix + "@example.invalid"
	if _, err := admin.Exec(ctx, `
INSERT INTO identity.users (id, email, email_normalized, display_name, password_hash)
VALUES ($1, $2, $2, 'Webhook integration', 'not-used')`, userID.PG(), email); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, userID.PG())
	}()
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.workspaces (id, name, slug)
VALUES ($1, 'Webhook integration', $2)`, workspaceID.PG(), "webhook-"+suffix[:24]); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.memberships (workspace_id, id, user_id, role)
VALUES ($1, $2, $3, 'owner')`, workspaceID.PG(), membershipID.PG(), userID.PG()); err != nil {
		t.Fatal(err)
	}

	handler := NewWebhookDispatchHandler(NewPostgresRepository(app))
	automationEventID := mustWebhookWorkerID(t)
	insertWebhookTestOutboxEvent(t, ctx, admin, workspaceID, automationEventID,
		"automation.scheduled", "workspace", workspaceID)
	automationJob := webhookDispatchTestJob(
		t, workspaceID, automationEventID, "automation.scheduled", "workspace", workspaceID,
	)
	for attempt := 1; attempt <= 2; attempt++ {
		if err := handler(context.Background(), worker.Dependencies{}, automationJob); err != nil {
			t.Fatalf("dispatch automation event without subscriptions, attempt %d: %v", attempt, err)
		}
	}
	assertWebhookTestCounts(t, ctx, admin, workspaceID, 0, 0)

	subscriptionID := mustWebhookWorkerID(t)
	if _, err := admin.Exec(ctx, `
INSERT INTO integrations.webhook_subscriptions (
  workspace_id, id, url, event_types, secret_ciphertext, secret_nonce, key_id, created_by
) VALUES ($1, $2, 'https://hooks.example.test/events', $3, $4, $5, 'test-key', $6)`,
		workspaceID.PG(), subscriptionID.PG(), []string{"customers.contact.created"},
		[]byte("ciphertext"), []byte("nonce"), userID.PG()); err != nil {
		t.Fatal(err)
	}
	contactEventID := mustWebhookWorkerID(t)
	contactID := mustWebhookWorkerID(t)
	insertWebhookTestOutboxEvent(t, ctx, admin, workspaceID, contactEventID,
		"customers.contact.created", "contact", contactID)
	contactJob := webhookDispatchTestJob(
		t, workspaceID, contactEventID, "customers.contact.created", "contact", contactID,
	)
	for attempt := 1; attempt <= 2; attempt++ {
		if err := handler(context.Background(), worker.Dependencies{}, contactJob); err != nil {
			t.Fatalf("dispatch subscribed contact event, attempt %d: %v", attempt, err)
		}
	}
	assertWebhookTestCounts(t, ctx, admin, workspaceID, 1, 1)

	var deliveryID, queuedDeliveryID, deliveryStatus, jobState string
	if err := admin.QueryRow(ctx, `
SELECT delivery.id::text, job.payload->>'deliveryId', delivery.status, job.state
FROM integrations.webhook_deliveries AS delivery
JOIN platform.jobs AS job
  ON job.workspace_id = delivery.workspace_id
 AND job.kind = 'webhook.deliver'
WHERE delivery.workspace_id = $1 AND delivery.event_id = $2`,
		workspaceID.PG(), contactEventID.PG(),
	).Scan(&deliveryID, &queuedDeliveryID, &deliveryStatus, &jobState); err != nil {
		t.Fatal(err)
	}
	if deliveryID != queuedDeliveryID || deliveryStatus != "queued" || jobState != "ready" {
		t.Fatalf("delivery=%q queued=%q status=%q job=%q", deliveryID, queuedDeliveryID, deliveryStatus, jobState)
	}
}

func insertWebhookTestOutboxEvent(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	workspaceID, eventID ids.UUID,
	eventType, aggregateType string,
	aggregateID ids.UUID,
) {
	t.Helper()
	if _, err := admin.Exec(ctx, `
INSERT INTO platform.outbox_events (
  workspace_id, id, event_type, aggregate_type, aggregate_id, correlation_id, payload
) VALUES ($1, $2, $3, $4, $5, $2, '{}'::jsonb)`,
		workspaceID.PG(), eventID.PG(), eventType, aggregateType, aggregateID.PG()); err != nil {
		t.Fatal(err)
	}
}

func webhookDispatchTestJob(
	t *testing.T,
	workspaceID, eventID ids.UUID,
	eventType, aggregateType string,
	aggregateID ids.UUID,
) worker.Job {
	t.Helper()
	payload, err := json.Marshal(webhookDispatchPointer{
		OutboxEventID: eventID.String(), EventType: eventType, SchemaVersion: 1,
		AggregateType: aggregateType, AggregateID: aggregateID.String(), CorrelationID: eventID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return worker.Job{
		WorkspaceID: workspaceID, ID: mustWebhookWorkerID(t), Kind: "webhook.dispatch",
		SchemaVersion: 1, Payload: payload,
	}
}

func assertWebhookTestCounts(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	workspaceID ids.UUID,
	wantDeliveries, wantJobs int,
) {
	t.Helper()
	var deliveries, jobs int
	if err := admin.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM integrations.webhook_deliveries WHERE workspace_id=$1),
  (SELECT count(*) FROM platform.jobs WHERE workspace_id=$1 AND kind='webhook.deliver')`,
		workspaceID.PG()).Scan(&deliveries, &jobs); err != nil {
		t.Fatal(err)
	}
	if deliveries != wantDeliveries || jobs != wantJobs {
		t.Fatalf("webhook fanout counts deliveries=%d jobs=%d, want %d/%d", deliveries, jobs, wantDeliveries, wantJobs)
	}
}
