package search

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

type recordingReconciler struct {
	called  int
	pointer eventPointer
}

func (reconciler *recordingReconciler) Reconcile(_ context.Context, _ worker.Job, pointer eventPointer) error {
	reconciler.called++
	reconciler.pointer = pointer
	return nil
}

func TestSearchWorkerValidatesAndReconcilesOutboxPointer(t *testing.T) {
	eventID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	entityID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	payload, _ := json.Marshal(eventPointer{
		OutboxEventID: eventID.String(), EventType: "customers.contact.updated", SchemaVersion: 1,
		AggregateType: "contact", AggregateID: entityID.String(), CorrelationID: eventID.String(),
	})
	reconciler := &recordingReconciler{}
	err := NewWorkerHandler(reconciler)(context.Background(), worker.Dependencies{}, worker.Job{
		SchemaVersion: 1, Payload: payload,
	})
	if err != nil {
		t.Fatalf("handle search job: %v", err)
	}
	if reconciler.called != 1 || reconciler.pointer.AggregateID != entityID.String() {
		t.Fatalf("unexpected reconciliation: calls=%d pointer=%+v", reconciler.called, reconciler.pointer)
	}
}

func TestSearchWorkerRejectsUnboundedOrUnknownPayload(t *testing.T) {
	reconciler := &recordingReconciler{}
	handler := NewWorkerHandler(reconciler)
	for _, job := range []worker.Job{
		{SchemaVersion: 2, Payload: []byte(`{}`)},
		{SchemaVersion: 1, Payload: make([]byte, maxSearchJobPayload+1)},
		{SchemaVersion: 1, Payload: []byte(`{"outboxEventId":"bad"}`)},
		{SchemaVersion: 1, Payload: []byte(`{"outboxEventId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b4","schemaVersion":1,"aggregateType":"contact","aggregateId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b5","extra":true}`)},
	} {
		if err := handler(context.Background(), worker.Dependencies{}, job); err == nil {
			t.Fatal("expected invalid payload error")
		}
	}
	if reconciler.called != 0 {
		t.Fatalf("invalid jobs reached reconciler %d times", reconciler.called)
	}
}
