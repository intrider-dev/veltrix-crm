package notifications

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

type recordingDispatchRepository struct {
	called  int
	pointer dispatchPointer
}

func (repository *recordingDispatchRepository) Dispatch(_ context.Context, _ worker.Job, pointer dispatchPointer) error {
	repository.called++
	repository.pointer = pointer
	return nil
}

func TestNotificationDispatchValidatesPointerBeforeRepository(t *testing.T) {
	eventID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	dealID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	payload, _ := json.Marshal(dispatchPointer{
		OutboxEventID: eventID.String(), EventType: "sales.deal.stage_changed", SchemaVersion: 1,
		AggregateType: "deal", AggregateID: dealID.String(), CorrelationID: eventID.String(),
	})
	repository := &recordingDispatchRepository{}
	err := NewDispatchWorkerHandler(repository)(context.Background(), worker.Dependencies{}, worker.Job{
		SchemaVersion: 1, Payload: payload,
	})
	if err != nil {
		t.Fatalf("dispatch notification: %v", err)
	}
	if repository.called != 1 || repository.pointer.EventType != "sales.deal.stage_changed" {
		t.Fatalf("unexpected dispatch: calls=%d pointer=%+v", repository.called, repository.pointer)
	}
}

func TestNotificationDispatchRejectsUnknownAndOversizedPayloads(t *testing.T) {
	repository := &recordingDispatchRepository{}
	handler := NewDispatchWorkerHandler(repository)
	for _, job := range []worker.Job{
		{SchemaVersion: 2, Payload: []byte(`{}`)},
		{SchemaVersion: 1, Payload: make([]byte, maxDispatchJobPayload+1)},
		{SchemaVersion: 1, Payload: []byte(`{"outboxEventId":"bad"}`)},
		{SchemaVersion: 1, Payload: []byte(`{"outboxEventId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b4","eventType":"sales.deal.stage_changed","schemaVersion":1,"aggregateType":"deal","aggregateId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b5","unexpected":true}`)},
	} {
		if err := handler(context.Background(), worker.Dependencies{}, job); err == nil {
			t.Fatal("expected invalid dispatch payload error")
		}
	}
	if repository.called != 0 {
		t.Fatalf("invalid jobs reached repository %d times", repository.called)
	}
}

func TestActivityNotificationKeysRemainEventSpecific(t *testing.T) {
	want := map[string]string{
		"activities.activity.created":   "notifications.activity.assigned",
		"activities.activity.updated":   "notifications.activity.updated",
		"activities.activity.completed": "notifications.activity.completed",
		"activities.activity.deleted":   "notifications.activity.deleted",
	}
	for eventType, key := range want {
		mapped, ok := activityNotificationMessageKey(eventType)
		if !ok || mapped != key {
			t.Fatalf("event %s mapped to %s", eventType, mapped)
		}
	}
	if _, ok := activityNotificationMessageKey("activities.comment.created"); ok {
		t.Fatal("comment event was accepted as an activity assignment event")
	}
}
