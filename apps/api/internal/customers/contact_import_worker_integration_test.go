//go:build integration

package customers_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestContactImportWorkerCompletesWithBoundedSSEExpiry(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "contact-import-worker-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	app, err := database.Open(ctx, appURL, 2, "contact-import-worker-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	workspaceID := mustContactImportWorkerID(t)
	userID := mustContactImportWorkerID(t)
	membershipID := mustContactImportWorkerID(t)
	sessionID := mustContactImportWorkerID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")
	fixtureEmail := "imported-" + suffix + "@example.invalid"
	userEmail := "import-worker-" + suffix + "@example.invalid"

	if _, err := admin.Exec(ctx, `
INSERT INTO identity.users (id, email, email_normalized, display_name, password_hash)
VALUES ($1, $2, $2, 'Contact import worker integration', 'not-used')`, userID.PG(), userEmail); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, userID.PG())
	}()
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.workspaces (id, name, slug)
VALUES ($1, 'Contact import worker integration', $2)`, workspaceID.PG(), "import-worker-"+suffix[:24]); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.memberships (workspace_id, id, user_id, role)
VALUES ($1, $2, $3, 'owner')`, workspaceID.PG(), membershipID.PG(), userID.PG()); err != nil {
		t.Fatal(err)
	}

	mapping, err := json.Marshal(customers.ContactImportMapping{
		FirstName: "First name",
		LastName:  "Last name",
		Email:     "Email",
	})
	if err != nil {
		t.Fatal(err)
	}
	values, err := json.Marshal(map[string]string{
		"First name": "Import",
		"Last name":  "Regression",
		"Email":      fixtureEmail,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO customers.import_sessions (
  workspace_id, id, actor_user_id, entity_type, status, mapping, source_headers, total_rows
) VALUES ($1, $2, $3, 'contact', 'queued', $4, '["First name","Last name","Email"]'::jsonb, 1)`,
		workspaceID.PG(), sessionID.PG(), userID.PG(), mapping); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO customers.import_rows (
  workspace_id, import_session_id, row_number, source_values, state
) VALUES ($1, $2, 2, $3, 'pending')`, workspaceID.PG(), sessionID.PG(), values); err != nil {
		t.Fatal(err)
	}

	service := customers.NewService()
	payload := customers.ContactImportJobPayload{
		ImportSessionID: sessionID.String(),
		ActorUserID:     userID.String(),
	}
	if err := service.ProcessContactImportJob(ctx, app, workspaceID, payload); err != nil {
		t.Fatalf("process queued contact import: %v", err)
	}
	// A retry after commit must observe the completed checkpoint and remain a no-op.
	if err := service.ProcessContactImportJob(ctx, app, workspaceID, payload); err != nil {
		t.Fatalf("retry completed contact import: %v", err)
	}

	var (
		status                                  string
		processedRows, createdRows, errorRows   int
		completedAt                             *time.Time
		contactID, displayName, normalizedEmail string
		rowState, rowContactID                  string
	)
	if err := admin.QueryRow(ctx, `
SELECT session.status, session.processed_rows, session.created_rows, session.error_rows,
       session.completed_at, contact.id::text, contact.display_name, contact.email_normalized,
       import_row.state, import_row.created_entity_id::text
FROM customers.import_sessions AS session
JOIN customers.import_rows AS import_row
  ON import_row.workspace_id = session.workspace_id
 AND import_row.import_session_id = session.id
JOIN customers.contacts AS contact
  ON contact.workspace_id = import_row.workspace_id
 AND contact.id = import_row.created_entity_id
WHERE session.workspace_id = $1 AND session.id = $2 AND import_row.row_number = 2`,
		workspaceID.PG(), sessionID.PG()).Scan(
		&status, &processedRows, &createdRows, &errorRows, &completedAt,
		&contactID, &displayName, &normalizedEmail, &rowState, &rowContactID,
	); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || processedRows != 1 || createdRows != 1 || errorRows != 0 || completedAt == nil {
		t.Fatalf("import status=%q processed=%d created=%d errors=%d completedAt=%v",
			status, processedRows, createdRows, errorRows, completedAt)
	}
	if contactID != rowContactID || rowState != "created" || displayName != "Import Regression" || normalizedEmail != fixtureEmail {
		t.Fatalf("contact=%q rowContact=%q rowState=%q displayName=%q email=%q",
			contactID, rowContactID, rowState, displayName, normalizedEmail)
	}

	var eventCount int
	var allExpiriesPresent, allExpiriesBounded bool
	if err := admin.QueryRow(ctx, `
SELECT count(*),
       bool_and(expires_at IS NOT NULL),
       bool_and(
         expires_at >= created_at + interval '23 hours 59 minutes'
         AND expires_at <= created_at + interval '24 hours 1 minute'
       )
FROM notifications.sse_events
WHERE workspace_id = $1 AND data->>'importSessionId' = $2`,
		workspaceID.PG(), sessionID.String()).Scan(
		&eventCount, &allExpiriesPresent, &allExpiriesBounded,
	); err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 || !allExpiriesPresent || !allExpiriesBounded {
		t.Fatalf("SSE events=%d nonNullExpiry=%t boundedExpiry=%t, want 2/true/true",
			eventCount, allExpiriesPresent, allExpiriesBounded)
	}
}

func mustContactImportWorkerID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
