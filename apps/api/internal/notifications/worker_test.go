package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func TestWorkerHandlersRegistersBoundedDeliveryKinds(t *testing.T) {
	t.Parallel()
	handlers := WorkerHandlers(nil, nil)
	if len(handlers) != 2 || handlers["activity.reminder"] == nil || handlers["notification.email"] == nil {
		t.Fatalf("worker handlers = %v", handlers)
	}
}

func TestDecodeJobPayloadBindsRecipientAndSchema(t *testing.T) {
	t.Parallel()
	recipient := ids.MustParse("018f0000-0000-7000-8000-000000000001")
	reminder := ids.MustParse("018f0000-0000-7000-8000-000000000002")
	payload, err := json.Marshal(jobPayload{ReminderID: reminder.String(), RecipientID: recipient.String()})
	if err != nil {
		t.Fatal(err)
	}
	decoded, gotRecipient, err := decodeJobPayload(worker.Job{SchemaVersion: 1, Payload: payload}, true)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ReminderID != reminder.String() || gotRecipient != recipient {
		t.Fatalf("decoded payload = %+v, recipient = %s", decoded, gotRecipient)
	}
	if _, _, err := decodeJobPayload(worker.Job{SchemaVersion: 2, Payload: payload}, true); err == nil {
		t.Fatal("unsupported job schema accepted")
	}
}

func TestEmailHandlerWithoutTransportReturnsStableFailureCode(t *testing.T) {
	t.Parallel()
	recipient := ids.MustParse("018f0000-0000-7000-8000-000000000001")
	notification := ids.MustParse("018f0000-0000-7000-8000-000000000002")
	payload, err := json.Marshal(jobPayload{
		NotificationID: notification.String(), RecipientID: recipient.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := WorkerHandlers(nil, nil)["notification.email"]
	err = handler(t.Context(), worker.Dependencies{}, worker.Job{
		WorkspaceID:   ids.MustParse("018f0000-0000-7000-8000-000000000003"),
		ID:            ids.MustParse("018f0000-0000-7000-8000-000000000004"),
		SchemaVersion: 1, Payload: payload, LockedUntil: time.Now().Add(time.Minute),
	})
	failure, ok := err.(interface{ FailureCode() string })
	if !ok || failure.FailureCode() != "email_not_configured" {
		t.Fatalf("error = %#v", err)
	}
}
