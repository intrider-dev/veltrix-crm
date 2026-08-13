package broker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

const validID = "018f47d2-2044-7f25-89b0-85bd4c8ad8b4"

func TestEnvelopeIsStrictBoundedAndPIIFree(t *testing.T) {
	envelope := Envelope{EventID: validID, WorkspaceID: validID, EventType: "sales.deal.updated", SchemaVersion: 1, AggregateType: "deal", AggregateID: validID, CorrelationID: validID}
	encoded, err := envelope.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvelope(append(encoded[:len(encoded)-1], []byte(`,"contactEmail":"private@example.test"}`)...)); err == nil {
		t.Fatal("unknown PII field was accepted")
	}
	if _, err := ParseEnvelope([]byte(`{"eventId":"bad"}`)); err == nil {
		t.Fatal("malformed envelope was accepted")
	}
}

type capturePublisher struct{ envelope Envelope }

func (publisher *capturePublisher) Publish(_ context.Context, envelope Envelope) error {
	publisher.envelope = envelope
	return nil
}
func (*capturePublisher) Close() error { return nil }

func TestPublishHandlerFailsClosedAcrossTenants(t *testing.T) {
	workspaceID, _ := ids.Parse(validID)
	otherID, _ := ids.Parse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	envelope := Envelope{EventID: validID, WorkspaceID: otherID.String(), EventType: "sales.deal.updated", SchemaVersion: 1, AggregateType: "deal", AggregateID: validID, CorrelationID: validID}
	payload, _ := json.Marshal(envelope)
	handler := NewPublishHandler(&capturePublisher{})
	err := handler(context.Background(), worker.Dependencies{}, worker.Job{WorkspaceID: workspaceID, IdempotencyKey: validID, Payload: payload})
	if err == nil {
		t.Fatal("cross-tenant broker job was accepted")
	}
}
