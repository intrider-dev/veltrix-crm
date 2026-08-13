package mailbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func TestDeliveryJobDoesNotResubmitAfterStatusCommitFailure(t *testing.T) {
	t.Parallel()
	job := deliveryTestJob(t, 1)
	store := &memoryDeliveryStore{state: "queued", sentCommitFailures: 1}
	sends := 0
	messageIDs := []string{}
	handler := newDeliveryJobHandler(store, func(_ context.Context, plan *deliveryPlan) error {
		sends++
		messageIDs = append(messageIDs, plan.outgoing.InternetMessageID)
		return nil
	})

	if err := handler(context.Background(), worker.Dependencies{}, job); err == nil {
		t.Fatal("expected persisted sent-state commit failure")
	}
	if store.state != "sending" || sends != 1 {
		t.Fatalf("after commit failure state=%q sends=%d, want sending/1", store.state, sends)
	}

	job.Attempts = 2
	if err := handler(context.Background(), worker.Dependencies{}, job); err != nil {
		t.Fatalf("retry after ambiguous submission: %v", err)
	}
	if store.state != "dead" || store.lastCode != "mail_delivery_uncertain" || sends != 1 {
		t.Fatalf("retry state=%q code=%q sends=%d", store.state, store.lastCode, sends)
	}
	if len(messageIDs) != 1 || messageIDs[0] != "stable-message@example.test" {
		t.Fatalf("SMTP received unstable message IDs: %#v", messageIDs)
	}
}

func TestDeliveryJobRetriesOnlySafePreSubmissionFailure(t *testing.T) {
	t.Parallel()
	job := deliveryTestJob(t, 1)
	store := &memoryDeliveryStore{state: "queued"}
	sends := 0
	handler := newDeliveryJobHandler(store, func(_ context.Context, _ *deliveryPlan) error {
		sends++
		if sends == 1 {
			return ErrEndpointUnavailable
		}
		return nil
	})

	if err := handler(context.Background(), worker.Dependencies{}, job); err == nil {
		t.Fatal("expected retryable pre-submission failure")
	}
	if store.state != "failed" || store.lastCode != "mail_endpoint_unavailable" {
		t.Fatalf("first attempt state=%q code=%q", store.state, store.lastCode)
	}
	job.Attempts = 2
	if err := handler(context.Background(), worker.Dependencies{}, job); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if store.state != "sent" || sends != 2 {
		t.Fatalf("second attempt state=%q sends=%d", store.state, sends)
	}
}

func TestDeliveryJobNeverRetriesAmbiguousSMTPFailure(t *testing.T) {
	t.Parallel()
	job := deliveryTestJob(t, 1)
	store := &memoryDeliveryStore{state: "queued"}
	sends := 0
	handler := newDeliveryJobHandler(store, func(_ context.Context, _ *deliveryPlan) error {
		sends++
		return ErrSMTPSubmissionUncertain
	})

	if err := handler(context.Background(), worker.Dependencies{}, job); err != nil {
		t.Fatalf("ambiguous outcome should be terminal without requeue: %v", err)
	}
	if store.state != "dead" || store.lastCode != "mail_delivery_uncertain" || sends != 1 {
		t.Fatalf("ambiguous outcome state=%q code=%q sends=%d", store.state, store.lastCode, sends)
	}
}

type memoryDeliveryStore struct {
	state              string
	lastCode           string
	sentCommitFailures int
}

func (store *memoryDeliveryStore) prepare(
	_ context.Context, job worker.Job, payload deliveryJobPayload,
) (*deliveryPlan, bool, error) {
	actorID, _ := ids.Parse(payload.ActorID)
	outgoingID, _ := ids.Parse(payload.OutgoingID)
	switch store.state {
	case "sent", "dead":
		return nil, true, nil
	case "sending":
		store.state = "dead"
		store.lastCode = "mail_delivery_uncertain"
		return nil, true, nil
	case "queued", "failed":
		store.state = "sending"
		return &deliveryPlan{
			workspaceID: job.WorkspaceID,
			actorID:     actorID,
			outgoingID:  outgoingID,
			outgoing: dbgen.MailboxOutgoingMessage{
				InternetMessageID: "stable-message@example.test",
			},
			password: []byte("secret"),
		}, false, nil
	default:
		return nil, false, errors.New("unexpected state")
	}
}

func (store *memoryDeliveryStore) markSent(
	_ context.Context, _ worker.Job, _ *deliveryPlan,
) error {
	if store.sentCommitFailures > 0 {
		store.sentCommitFailures--
		return errors.New("simulated commit failure")
	}
	store.state = "sent"
	store.lastCode = ""
	return nil
}

func (store *memoryDeliveryStore) markFailed(
	_ context.Context, _ worker.Job, _ *deliveryPlan, code string, terminal bool,
) error {
	store.lastCode = code
	if terminal {
		store.state = "dead"
	} else {
		store.state = "failed"
	}
	return nil
}

func deliveryTestJob(t *testing.T, attempts int32) worker.Job {
	t.Helper()
	workspaceID, outgoingID, actorID := mustTestID(t), mustTestID(t), mustTestID(t)
	payload, err := json.Marshal(deliveryJobPayload{OutgoingID: outgoingID.String(), ActorID: actorID.String()})
	if err != nil {
		t.Fatal(err)
	}
	return worker.Job{
		WorkspaceID: workspaceID, ID: mustTestID(t), Kind: DeliveryJobKind,
		SchemaVersion: 1, Payload: payload, Attempts: attempts, MaxAttempts: 5,
	}
}
