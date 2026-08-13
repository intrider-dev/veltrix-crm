package broker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const (
	SchemaVersion  = 1
	MaxMessageSize = 256 << 10
)

// Envelope deliberately carries identifiers rather than CRM record content.
// Consumers must reload canonical state under transaction-local tenant context.
type Envelope struct {
	EventID       string          `json:"eventId"`
	WorkspaceID   string          `json:"workspaceId"`
	EventType     string          `json:"eventType"`
	SchemaVersion int32           `json:"schemaVersion"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	CorrelationID string          `json:"correlationId"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

func (envelope Envelope) Validate() error {
	if _, err := ids.Parse(envelope.EventID); err != nil {
		return errors.New("event ID is invalid")
	}
	if _, err := ids.Parse(envelope.WorkspaceID); err != nil {
		return errors.New("workspace ID is invalid")
	}
	if _, err := ids.Parse(envelope.AggregateID); err != nil {
		return errors.New("aggregate ID is invalid")
	}
	if _, err := ids.Parse(envelope.CorrelationID); err != nil {
		return errors.New("correlation ID is invalid")
	}
	if envelope.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", envelope.SchemaVersion)
	}
	if !safeName(envelope.EventType, 160) || !safeName(envelope.AggregateType, 80) {
		return errors.New("event or aggregate type is invalid")
	}
	if len(envelope.Payload) > MaxMessageSize || !utf8.Valid(envelope.Payload) {
		return errors.New("payload exceeds the broker contract")
	}
	return nil
}

func (envelope Envelope) Marshal() ([]byte, error) {
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode broker envelope: %w", err)
	}
	if len(encoded) > MaxMessageSize {
		return nil, errors.New("encoded broker envelope is too large")
	}
	return encoded, nil
}

func ParseEnvelope(data []byte) (Envelope, error) {
	if len(data) == 0 || len(data) > MaxMessageSize || !utf8.Valid(data) {
		return Envelope{}, errors.New("broker envelope size or encoding is invalid")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, errors.New("broker envelope is invalid")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Envelope{}, errors.New("broker envelope contains trailing data")
	}
	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func (envelope Envelope) PartitionKey() string {
	return envelope.WorkspaceID + "/" + envelope.AggregateType + "/" + envelope.AggregateID
}

func safeName(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if !(character == '.' || character == '-' || character == '_' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}
