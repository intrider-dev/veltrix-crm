package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Metadata struct {
	WorkspaceID   ids.UUID
	ActorID       ids.UUID
	RequestID     string
	CorrelationID ids.UUID
	IPAddress     *netip.Addr
	UserAgent     string
}

type Mutation struct {
	Action        string
	EventType     string
	AggregateType string
	AggregateID   ids.UUID
	Summary       map[string]any
	Payload       map[string]any
}

func Record(ctx context.Context, queries *dbgen.Queries, metadata Metadata, mutation Mutation) error {
	payload, err := recordAudit(ctx, queries, &metadata, mutation)
	if err != nil {
		return err
	}
	outboxID, err := ids.NewV7()
	if err != nil {
		return err
	}
	if err := queries.InsertOutboxEvent(ctx, dbgen.InsertOutboxEventParams{
		WorkspaceID:   metadata.WorkspaceID.PG(),
		ID:            outboxID.PG(),
		EventType:     mutation.EventType,
		SchemaVersion: 1,
		AggregateType: mutation.AggregateType,
		AggregateID:   mutation.AggregateID.PG(),
		CausationID:   pgtype.UUID{},
		CorrelationID: metadata.CorrelationID.PG(),
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return insertSSE(ctx, queries, metadata.WorkspaceID, mutation.EventType, payload, nil)
}

// RecordTargeted persists an audit record and one user-addressed SSE event per
// recipient. It intentionally creates no generic outbox event: workspace-wide
// automation and webhook fan-out must never receive restricted record data.
func RecordTargeted(
	ctx context.Context,
	queries *dbgen.Queries,
	metadata Metadata,
	mutation Mutation,
	recipients []ids.UUID,
) error {
	payload, err := recordAudit(ctx, queries, &metadata, mutation)
	if err != nil {
		return err
	}
	seen := make(map[ids.UUID]struct{}, len(recipients))
	for _, recipient := range recipients {
		if recipient == (ids.UUID{}) {
			continue
		}
		if _, exists := seen[recipient]; exists {
			continue
		}
		seen[recipient] = struct{}{}
		if err := insertSSE(ctx, queries, metadata.WorkspaceID, mutation.EventType, payload, &recipient); err != nil {
			return err
		}
	}
	if len(seen) == 0 {
		return fmt.Errorf("targeted event requires at least one recipient")
	}
	return nil
}

func recordAudit(
	ctx context.Context, queries *dbgen.Queries, metadata *Metadata, mutation Mutation,
) ([]byte, error) {
	auditID, err := ids.NewV7()
	if err != nil {
		return nil, err
	}
	if metadata.CorrelationID == (ids.UUID{}) {
		metadata.CorrelationID, err = ids.NewV7()
		if err != nil {
			return nil, err
		}
	}
	summary, err := json.Marshal(mutation.Summary)
	if err != nil {
		return nil, fmt.Errorf("encode audit summary: %w", err)
	}
	payload, err := json.Marshal(mutation.Payload)
	if err != nil {
		return nil, fmt.Errorf("encode event payload: %w", err)
	}
	if len(metadata.UserAgent) > 512 {
		metadata.UserAgent = metadata.UserAgent[:512]
	}
	if err := queries.InsertAuditEvent(ctx, dbgen.InsertAuditEventParams{
		WorkspaceID: metadata.WorkspaceID.PG(),
		ID:          auditID.PG(),
		ActorUserID: metadata.ActorID.PG(),
		Action:      mutation.Action,
		EntityType:  mutation.AggregateType,
		EntityID:    mutation.AggregateID.PG(),
		RequestID:   metadata.RequestID,
		Summary:     summary,
		IpAddress:   metadata.IPAddress,
		UserAgent:   metadata.UserAgent,
	}); err != nil {
		return nil, fmt.Errorf("insert audit event: %w", err)
	}
	return payload, nil
}

func insertSSE(
	ctx context.Context, queries *dbgen.Queries, workspaceID ids.UUID,
	eventType string, payload []byte, recipient *ids.UUID,
) error {
	sseID, err := ids.NewV7()
	if err != nil {
		return err
	}
	if recipient != nil {
		if err := queries.InsertUserSSEEvent(ctx, dbgen.InsertUserSSEEventParams{
			WorkspaceID: workspaceID.PG(), EventID: sseID.PG(), EventType: eventType,
			Data: payload, RecipientUserID: recipient.PG(),
		}); err != nil {
			return fmt.Errorf("insert targeted SSE event: %w", err)
		}
		return nil
	}
	if err := queries.InsertSSEEvent(ctx, dbgen.InsertSSEEventParams{
		WorkspaceID: workspaceID.PG(),
		ID:          sseID.PG(),
		EventType:   eventType,
		Data:        payload,
	}); err != nil {
		return fmt.Errorf("insert SSE event: %w", err)
	}
	return nil
}
