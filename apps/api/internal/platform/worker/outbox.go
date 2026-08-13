package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type eventPointer struct {
	OutboxEventID string `json:"outboxEventId"`
	EventType     string `json:"eventType"`
	SchemaVersion int32  `json:"schemaVersion"`
	AggregateType string `json:"aggregateType"`
	AggregateID   string `json:"aggregateId"`
	CorrelationID string `json:"correlationId"`
}

type outboxFailure struct {
	workspaceID ids.UUID
	eventID     ids.UUID
	attempts    int32
}

func (runtime *runtime) dispatchOutboxBatch(ctx context.Context) (int, *outboxFailure, error) {
	tx, err := runtime.config.DispatcherPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("begin outbox transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := dbgen.New(tx)
	events, err := queries.ClaimOutboxEvents(ctx, runtime.config.OutboxBatchSize)
	if err != nil {
		return 0, nil, fmt.Errorf("claim outbox events: %w", err)
	}
	for _, event := range events {
		failure, fanoutErr := runtime.fanoutEvent(ctx, queries, event)
		if fanoutErr != nil {
			return 0, failure, fanoutErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, fmt.Errorf("commit outbox transaction: %w", err)
	}
	return len(events), nil, nil
}

func (runtime *runtime) fanoutEvent(ctx context.Context, queries *dbgen.Queries, event dbgen.ClaimOutboxEventsRow) (*outboxFailure, error) {
	workspaceID, workspaceValid := ids.FromPG(event.WorkspaceID)
	eventID, eventValid := ids.FromPG(event.ID)
	aggregateID, aggregateValid := ids.FromPG(event.AggregateID)
	correlationID, correlationValid := ids.FromPG(event.CorrelationID)
	if !workspaceValid || !eventValid || !aggregateValid || !correlationValid {
		return nil, errors.New("outbox event contains an invalid UUID")
	}
	failure := &outboxFailure{workspaceID: workspaceID, eventID: eventID, attempts: event.Attempts}
	pointer, err := json.Marshal(eventPointer{
		OutboxEventID: eventID.String(),
		EventType:     event.EventType,
		SchemaVersion: event.SchemaVersion,
		AggregateType: event.AggregateType,
		AggregateID:   aggregateID.String(),
		CorrelationID: correlationID.String(),
	})
	if err != nil {
		return failure, fmt.Errorf("encode outbox pointer: %w", err)
	}
	for _, kind := range append(fanoutKinds(event.EventType, event.AggregateType, event.Payload), runtime.config.BrokerJobKinds...) {
		jobID, err := ids.NewV7()
		if err != nil {
			return failure, fmt.Errorf("generate fanout job ID: %w", err)
		}
		jobPayload := pointer
		if strings.HasPrefix(kind, "broker.") {
			jobPayload, err = json.Marshal(struct {
				EventID       string `json:"eventId"`
				WorkspaceID   string `json:"workspaceId"`
				EventType     string `json:"eventType"`
				SchemaVersion int32  `json:"schemaVersion"`
				AggregateType string `json:"aggregateType"`
				AggregateID   string `json:"aggregateId"`
				CorrelationID string `json:"correlationId"`
			}{eventID.String(), workspaceID.String(), event.EventType, event.SchemaVersion, event.AggregateType, aggregateID.String(), correlationID.String()})
			if err != nil {
				return failure, fmt.Errorf("encode broker event pointer: %w", err)
			}
		}
		if err := queries.InsertFanoutJob(ctx, dbgen.InsertFanoutJobParams{
			WorkspaceID:    event.WorkspaceID,
			ID:             jobID.PG(),
			Kind:           kind,
			SchemaVersion:  1,
			IdempotencyKey: eventID.String(),
			Payload:        jobPayload,
			MaxAttempts:    runtime.config.MaxAttempts,
		}); err != nil {
			return failure, fmt.Errorf("insert %s fanout job: %w", kind, err)
		}
	}
	updated, err := queries.MarkOutboxPublished(ctx, dbgen.MarkOutboxPublishedParams{
		WorkspaceID: event.WorkspaceID,
		ID:          event.ID,
	})
	if err != nil {
		return failure, fmt.Errorf("mark outbox event published: %w", err)
	}
	if updated != 1 {
		return failure, errors.New("outbox event publication lost its row lock")
	}
	return nil, nil
}

func fanoutKinds(eventType, aggregateType string, payload json.RawMessage) []string {
	kinds := make([]string, 0, 4)
	switch aggregateType {
	case "contact", "company", "lead", "deal":
		kinds = append(kinds, "search.sync")
	case "activity":
		var activity struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(payload, &activity) == nil && activity.Type == "note" {
			kinds = append(kinds, "search.sync")
		}
	}
	kinds = append(kinds, "automation.dispatch")
	if eventType == "sales.deal.stage_changed" || strings.HasPrefix(eventType, "activities.activity.") {
		kinds = append(kinds, "notification.dispatch")
	}
	kinds = append(kinds, "webhook.dispatch")
	return kinds
}

func (runtime *runtime) recordOutboxFailure(ctx context.Context, failure *outboxFailure) {
	if failure == nil || ctx.Err() != nil {
		return
	}
	delay := Backoff(failure.attempts+1, runtime.config.BackoffBase, runtime.config.BackoffMaximum)
	errorCode := "outbox_fanout_failed"
	operationCtx, cancel := context.WithTimeout(ctx, runtime.config.OperationTimeout)
	defer cancel()
	err := dbgen.New(runtime.config.DispatcherPool).RecordOutboxFailure(operationCtx, dbgen.RecordOutboxFailureParams{
		DelayMilliseconds: delay.Milliseconds(),
		ErrorCode:         &errorCode,
		WorkspaceID:       failure.workspaceID.PG(),
		ID:                failure.eventID.PG(),
	})
	if err != nil {
		runtime.config.Logger.Error("worker outbox failure could not be recorded",
			"component", "outbox",
			"event_id", failure.eventID.String(),
			"error_code", "record_failure_failed",
		)
	}
}
