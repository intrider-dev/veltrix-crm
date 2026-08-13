package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

const maxDispatchJobPayload = 8 << 10

type dispatchPointer struct {
	OutboxEventID string `json:"outboxEventId"`
	EventType     string `json:"eventType"`
	SchemaVersion int32  `json:"schemaVersion"`
	AggregateType string `json:"aggregateType"`
	AggregateID   string `json:"aggregateId"`
	CorrelationID string `json:"correlationId"`
}

type DispatchRepository interface {
	Dispatch(context.Context, worker.Job, dispatchPointer) error
}

func NewDispatchWorkerHandler(repository DispatchRepository) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if repository == nil {
			return codedJobError{"notification_not_configured", "notification dispatch repository is not configured"}
		}
		pointer, err := decodeDispatchPointer(job)
		if err != nil {
			return err
		}
		if err := repository.Dispatch(ctx, job, pointer); err != nil {
			return codedJobError{"notification_dispatch_failed", err.Error()}
		}
		return nil
	}
}

func decodeDispatchPointer(job worker.Job) (dispatchPointer, error) {
	if job.SchemaVersion != 1 || len(job.Payload) == 0 || len(job.Payload) > maxDispatchJobPayload {
		return dispatchPointer{}, codedJobError{"invalid_notification_payload", "notification dispatch payload is invalid"}
	}
	decoder := json.NewDecoder(bytes.NewReader(job.Payload))
	decoder.DisallowUnknownFields()
	var pointer dispatchPointer
	if err := decoder.Decode(&pointer); err != nil {
		return dispatchPointer{}, codedJobError{"invalid_notification_payload", "notification dispatch payload is invalid"}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return dispatchPointer{}, codedJobError{"invalid_notification_payload", "notification dispatch payload has trailing data"}
	}
	if pointer.OutboxEventID == "" || pointer.EventType == "" || pointer.SchemaVersion != 1 ||
		pointer.AggregateType == "" || pointer.AggregateID == "" {
		return dispatchPointer{}, codedJobError{"invalid_notification_payload", "notification dispatch pointer is incomplete"}
	}
	if _, err := ids.Parse(pointer.OutboxEventID); err != nil {
		return dispatchPointer{}, codedJobError{"invalid_notification_payload", "notification outbox ID is invalid"}
	}
	if _, err := ids.Parse(pointer.AggregateID); err != nil {
		return dispatchPointer{}, codedJobError{"invalid_notification_payload", "notification aggregate ID is invalid"}
	}
	return pointer, nil
}

type PostgresDispatchRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresDispatchRepository(pool *pgxpool.Pool) *PostgresDispatchRepository {
	return &PostgresDispatchRepository{pool: pool}
}

func (repository *PostgresDispatchRepository) Dispatch(
	ctx context.Context, job worker.Job, pointer dispatchPointer,
) error {
	if repository == nil || repository.pool == nil {
		return errors.New("notification database pool is required")
	}
	eventID, _ := ids.Parse(pointer.OutboxEventID)
	pointerAggregateID, _ := ids.Parse(pointer.AggregateID)
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin notification dispatch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: job.WorkspaceID.String(), RequestID: "notification:" + job.ID.String(),
	}); err != nil {
		return fmt.Errorf("set notification tenant context: %w", err)
	}
	event, err := queries.GetNotificationOutboxEvent(ctx, dbgen.GetNotificationOutboxEventParams{
		WorkspaceID: job.WorkspaceID.PG(), EventID: eventID.PG(),
	})
	if err != nil {
		return fmt.Errorf("load notification outbox event: %w", err)
	}
	aggregateID, valid := ids.FromPG(event.AggregateID)
	if !valid || event.SchemaVersion != pointer.SchemaVersion || event.EventType != pointer.EventType ||
		event.AggregateType != pointer.AggregateType || aggregateID != pointerAggregateID {
		return errors.New("notification job pointer does not match its tenant outbox event")
	}
	target, found, err := notificationTarget(ctx, queries, job.WorkspaceID, event.EventType, aggregateID, event.Payload)
	if err != nil {
		return err
	}
	if !found {
		// Unowned deals and unassigned activities intentionally do not generate
		// workspace-wide notifications. Disabled users are also skipped.
		return tx.Commit(ctx)
	}
	params, err := json.Marshal(target.params)
	if err != nil {
		return fmt.Errorf("encode notification params: %w", err)
	}
	entityType := target.entityType
	_, err = queries.CreateDispatchedNotification(ctx, dbgen.CreateDispatchedNotificationParams{
		WorkspaceID: job.WorkspaceID.PG(), NotificationID: eventID.PG(),
		RecipientUserID: target.recipientID.PG(), MessageKey: target.messageKey,
		MessageParams: params, EntityType: &entityType, EntityID: aggregateID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// The outbox event ID is the stable notification ID. A retry after a
		// committed transaction is therefore a true no-op, including SSE.
		return tx.Commit(ctx)
	}
	if err != nil {
		return fmt.Errorf("create dispatched notification: %w", err)
	}
	sseID, err := ids.NewV7()
	if err != nil {
		return err
	}
	data, err := json.Marshal(map[string]any{
		"id": eventID.String(), "recipientUserId": target.recipientID.String(),
		"messageKey": target.messageKey, "messageParams": target.params,
		"entityType": target.entityType, "entityId": aggregateID.String(),
	})
	if err != nil {
		return fmt.Errorf("encode notification SSE data: %w", err)
	}
	if err := queries.InsertUserSSEEvent(ctx, dbgen.InsertUserSSEEventParams{
		WorkspaceID: job.WorkspaceID.PG(), EventID: sseID.PG(),
		EventType: "notification.created", Data: data, RecipientUserID: target.recipientID.PG(),
	}); err != nil {
		return fmt.Errorf("insert notification SSE event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit notification dispatch: %w", err)
	}
	return nil
}

type dispatchTarget struct {
	recipientID ids.UUID
	messageKey  string
	params      map[string]any
	entityType  string
}

func notificationTarget(
	ctx context.Context, queries *dbgen.Queries, workspaceID ids.UUID, eventType string, entityID ids.UUID, payload []byte,
) (dispatchTarget, bool, error) {
	switch eventType {
	case "sales.deal.stage_changed":
		if len(payload) == 0 || len(payload) > 64<<10 {
			return dispatchTarget{}, false, errors.New("deal stage event payload is invalid")
		}
		var eventData struct {
			ToStageID string `json:"toStageId"`
		}
		if json.Unmarshal(payload, &eventData) != nil {
			return dispatchTarget{}, false, errors.New("deal stage event payload is invalid")
		}
		stageID, err := ids.Parse(eventData.ToStageID)
		if err != nil {
			return dispatchTarget{}, false, errors.New("deal stage event target is invalid")
		}
		row, err := queries.GetDealNotificationTarget(ctx, dbgen.GetDealNotificationTargetParams{
			WorkspaceID: workspaceID.PG(), EntityID: entityID.PG(), StageID: stageID.PG(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return dispatchTarget{}, false, nil
		}
		if err != nil {
			return dispatchTarget{}, false, fmt.Errorf("load deal notification target: %w", err)
		}
		return dispatchTarget{
			recipientID: requiredID(row.RecipientUserID), messageKey: "notifications.deal.stageChanged",
			params: map[string]any{"deal": row.Title, "stage": row.StageName}, entityType: "deal",
		}, true, nil
	case "activities.activity.created", "activities.activity.updated", "activities.activity.completed", "activities.activity.deleted":
		row, err := queries.GetActivityNotificationTarget(ctx, dbgen.GetActivityNotificationTargetParams{
			WorkspaceID: workspaceID.PG(), EntityID: entityID.PG(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return dispatchTarget{}, false, nil
		}
		if err != nil {
			return dispatchTarget{}, false, fmt.Errorf("load activity notification target: %w", err)
		}
		key, _ := activityNotificationMessageKey(eventType)
		return dispatchTarget{
			recipientID: requiredID(row.RecipientUserID), messageKey: key,
			params: map[string]any{"title": row.Title}, entityType: "activity",
		}, true, nil
	default:
		return dispatchTarget{}, false, fmt.Errorf("unsupported notification event %q", eventType)
	}
}

func activityNotificationMessageKey(eventType string) (string, bool) {
	switch eventType {
	case "activities.activity.created":
		return "notifications.activity.assigned", true
	case "activities.activity.updated":
		return "notifications.activity.updated", true
	case "activities.activity.completed":
		return "notifications.activity.completed", true
	case "activities.activity.deleted":
		return "notifications.activity.deleted", true
	default:
		return "", false
	}
}

var _ DispatchRepository = (*PostgresDispatchRepository)(nil)
