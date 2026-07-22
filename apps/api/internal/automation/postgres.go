package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

type dispatchPostgres struct {
	queries *dbgen.Queries
	event   Event
}

func NewDispatchWorkerHandler(pool *pgxpool.Pool) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if pool == nil {
			return executionFailure{code: "automation_not_configured", err: errors.New("automation database pool is required")}
		}
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		queries := dbgen.New(tx)
		if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
			WorkspaceID: job.WorkspaceID.String(), RequestID: job.ID.String(),
		}); err != nil {
			return err
		}
		var pointer struct {
			OutboxEventID string `json:"outboxEventId"`
		}
		if err := json.Unmarshal(job.Payload, &pointer); err != nil {
			return executionFailure{code: "automation_payload_invalid", err: err}
		}
		eventID, err := ids.Parse(pointer.OutboxEventID)
		if err != nil {
			return executionFailure{code: "automation_payload_invalid", err: err}
		}
		row, err := queries.GetAutomationOutboxEvent(ctx, dbgen.GetAutomationOutboxEventParams{
			WorkspaceID: job.WorkspaceID.PG(), ID: eventID.PG(),
		})
		if err != nil {
			return err
		}
		trigger, ok := triggerForEvent(row.EventType)
		if !ok {
			return tx.Commit(ctx)
		}
		aggregateID, ok := ids.FromPG(row.AggregateID)
		if !ok {
			return errors.New("automation event contains invalid aggregate ID")
		}
		correlationID, ok := ids.FromPG(row.CorrelationID)
		if !ok {
			return errors.New("automation event contains invalid correlation ID")
		}
		entityType := EntityType(row.AggregateType)
		if !validEntityType(entityType) {
			return tx.Commit(ctx)
		}
		fields, err := automationSnapshot(ctx, queries, job.WorkspaceID, entityType, aggregateID, row.Payload)
		if err != nil {
			return err
		}
		event := Event{
			WorkspaceID: job.WorkspaceID, EventID: eventID, CorrelationID: correlationID,
			Trigger: trigger, EntityType: entityType, EntityID: aggregateID, Fields: fields,
		}
		event.Depth = integerField(fields, "automation_depth")
		event.OwnerID = stringField(fields, "owner_user_id")
		event.TeamID = stringField(fields, "team_id")
		event.Tags = stringSliceField(fields, "tags")
		fields["entity_type"] = string(entityType)
		fields["entity_id"] = aggregateID.String()
		repository := &dispatchPostgres{queries: queries, event: event}
		if _, err := Dispatch(ctx, repository, event); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

func (repository *dispatchPostgres) ListEnabled(ctx context.Context, event Event) ([]Rule, error) {
	rows, err := repository.queries.ListEnabledAutomationRulesForTrigger(ctx, dbgen.ListEnabledAutomationRulesForTriggerParams{
		WorkspaceID: event.WorkspaceID.PG(), TriggerType: string(event.Trigger), EntityType: string(event.EntityType),
	})
	if err != nil {
		return nil, err
	}
	result := make([]Rule, 0, len(rows))
	for _, row := range rows {
		rule, err := ruleFromDB(row)
		if err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, nil
}

func (repository *dispatchPostgres) ReserveAndEnqueue(ctx context.Context, rule Rule, event Event) (QueueOutcome, error) {
	executionID, err := ids.NewV7()
	if err != nil {
		return QueueDuplicate, err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return QueueDuplicate, err
	}
	_, err = repository.queries.ReserveAutomationExecution(ctx, dbgen.ReserveAutomationExecutionParams{
		WorkspaceID: event.WorkspaceID.PG(), ID: executionID.PG(), RuleID: rule.ID.PG(),
		EventID: event.EventID.PG(), CorrelationID: event.CorrelationID.PG(), Depth: int32(event.Depth + 1),
		TriggerType: string(event.Trigger), EventPayload: payload,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return QueueDuplicate, nil
	}
	if err != nil {
		return QueueDuplicate, err
	}
	_, err = repository.queries.TryConsumeAutomationRateLimit(ctx, dbgen.TryConsumeAutomationRateLimitParams{
		WorkspaceID: rule.WorkspaceID.PG(), RuleID: rule.ID.PG(), RateLimitPerHour: int32(rule.Spec.RateLimitPerHour),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_, cleanupErr := repository.queries.CancelRateLimitedAutomationExecution(ctx, dbgen.CancelRateLimitedAutomationExecutionParams{
			WorkspaceID: event.WorkspaceID.PG(), ID: executionID.PG(),
		})
		if cleanupErr != nil {
			return QueueDuplicate, cleanupErr
		}
		return QueueRateLimited, nil
	}
	if err != nil {
		return QueueDuplicate, err
	}
	jobID, err := ids.NewV7()
	if err != nil {
		return QueueDuplicate, err
	}
	jobPayload, _ := json.Marshal(map[string]string{"executionId": executionID.String()})
	if err := repository.queries.EnqueueAutomationExecution(ctx, dbgen.EnqueueAutomationExecutionParams{
		WorkspaceID: event.WorkspaceID.PG(), ID: jobID.PG(), IdempotencyKey: executionID.String(),
		Payload: jobPayload, MaxAttempts: 8,
	}); err != nil {
		return QueueDuplicate, err
	}
	return QueueCreated, nil
}

func triggerForEvent(eventType string) (TriggerType, bool) {
	switch eventType {
	case "sales.deal.stage_changed":
		return TriggerDealStageChanged, true
	case "sales.deal.won":
		return TriggerDealWon, true
	case "sales.deal.lost":
		return TriggerDealLost, true
	case "activities.task.overdue":
		return TriggerTaskOverdue, true
	case "automation.scheduled":
		return TriggerScheduled, true
	}
	if strings.HasSuffix(eventType, ".created") {
		return TriggerRecordCreated, true
	}
	if strings.HasSuffix(eventType, ".updated") || strings.HasSuffix(eventType, ".completed") {
		return TriggerRecordUpdated, true
	}
	return "", false
}

func automationSnapshot(
	ctx context.Context, queries *dbgen.Queries, workspaceID ids.UUID, entityType EntityType, entityID ids.UUID, fallback []byte,
) (map[string]any, error) {
	var raw []byte
	var err error
	switch entityType {
	case EntityContact:
		raw, err = queries.GetContactAutomationSnapshot(ctx, dbgen.GetContactAutomationSnapshotParams{WorkspaceID: workspaceID.PG(), ID: entityID.PG()})
	case EntityCompany:
		raw, err = queries.GetCompanyAutomationSnapshot(ctx, dbgen.GetCompanyAutomationSnapshotParams{WorkspaceID: workspaceID.PG(), ID: entityID.PG()})
	case EntityLead:
		raw, err = queries.GetLeadAutomationSnapshot(ctx, dbgen.GetLeadAutomationSnapshotParams{WorkspaceID: workspaceID.PG(), ID: entityID.PG()})
	case EntityDeal:
		raw, err = queries.GetDealAutomationSnapshot(ctx, dbgen.GetDealAutomationSnapshotParams{WorkspaceID: workspaceID.PG(), ID: entityID.PG()})
	case EntityActivity:
		raw, err = queries.GetActivityAutomationSnapshot(ctx, dbgen.GetActivityAutomationSnapshotParams{WorkspaceID: workspaceID.PG(), ID: entityID.PG()})
	case EntityWorkspace:
		raw = fallback
	}
	if errors.Is(err, pgx.ErrNoRows) {
		raw, err = fallback, nil
	}
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode automation event snapshot: %w", err)
	}
	return fields, nil
}

func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func integerField(fields map[string]any, key string) int {
	switch value := fields[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func stringSliceField(fields map[string]any, key string) []string {
	values, _ := fields[key].([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

type ExecutionPostgres struct {
	pool *pgxpool.Pool
}

func NewExecutionPostgres(pool *pgxpool.Pool) *ExecutionPostgres {
	return &ExecutionPostgres{pool: pool}
}

func (repository *ExecutionPostgres) withTenant(
	ctx context.Context, workspaceID ids.UUID, requestID string, fn func(*dbgen.Queries) error,
) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{WorkspaceID: workspaceID.String(), RequestID: requestID}); err != nil {
		return err
	}
	if err := fn(queries); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *ExecutionPostgres) Begin(ctx context.Context, job worker.Job, rawID string) (Execution, bool, error) {
	id, err := ids.Parse(rawID)
	if err != nil {
		return Execution{}, false, err
	}
	var execution Execution
	claimed := false
	err = repository.withTenant(ctx, job.WorkspaceID, job.ID.String(), func(queries *dbgen.Queries) error {
		rows, err := queries.StartAutomationExecution(ctx, dbgen.StartAutomationExecutionParams{WorkspaceID: job.WorkspaceID.PG(), ID: id.PG()})
		if err != nil || rows == 0 {
			return err
		}
		executionRow, err := queries.GetAutomationExecution(ctx, dbgen.GetAutomationExecutionParams{WorkspaceID: job.WorkspaceID.PG(), ID: id.PG()})
		if err != nil {
			return err
		}
		ruleRow, err := queries.GetAutomationRule(ctx, dbgen.GetAutomationRuleParams{WorkspaceID: job.WorkspaceID.PG(), ID: executionRow.RuleID})
		if err != nil {
			return err
		}
		rule, err := ruleFromDB(ruleRow)
		if err != nil {
			return err
		}
		executionID, _ := ids.FromPG(executionRow.ID)
		ruleID, _ := ids.FromPG(executionRow.RuleID)
		eventID, _ := ids.FromPG(executionRow.EventID)
		correlationID, _ := ids.FromPG(executionRow.CorrelationID)
		execution = Execution{
			WorkspaceID: job.WorkspaceID, ID: executionID, RuleID: ruleID, EventID: eventID,
			CorrelationID: correlationID, Depth: int(executionRow.Depth), Trigger: TriggerType(executionRow.TriggerType),
			Actions: rule.Spec.Actions, Attempts: int(executionRow.Attempts), MaxAttempts: int(job.MaxAttempts), State: executionRow.State,
			ActorID: rule.CreatedBy,
		}
		if err := json.Unmarshal(executionRow.EventPayload, &execution.Event); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return execution, claimed, err
}

func (repository *ExecutionPostgres) BeginAction(
	ctx context.Context, execution Execution, index int, actionType ActionType, key string,
) (map[string]any, bool, error) {
	result := map[string]any{}
	claimed := false
	err := repository.withTenant(ctx, execution.WorkspaceID, execution.ID.String(), func(queries *dbgen.Queries) error {
		_, err := queries.StartAutomationAction(ctx, dbgen.StartAutomationActionParams{
			WorkspaceID: execution.WorkspaceID.PG(), ExecutionID: execution.ID.PG(), ActionIndex: int32(index),
			ActionType: string(actionType), IdempotencyKey: key,
		})
		if err == nil {
			claimed = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		previous, err := queries.GetAutomationAction(ctx, dbgen.GetAutomationActionParams{
			WorkspaceID: execution.WorkspaceID.PG(), ExecutionID: execution.ID.PG(), ActionIndex: int32(index),
		})
		if err != nil {
			return err
		}
		if previous.State != "completed" {
			return errors.New("automation action is already running")
		}
		return json.Unmarshal(previous.Result, &result)
	})
	return result, claimed, err
}

func (repository *ExecutionPostgres) CompleteAction(ctx context.Context, execution Execution, index int, result map[string]any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return repository.withTenant(ctx, execution.WorkspaceID, execution.ID.String(), func(queries *dbgen.Queries) error {
		rows, err := queries.CompleteAutomationAction(ctx, dbgen.CompleteAutomationActionParams{
			WorkspaceID: execution.WorkspaceID.PG(), ExecutionID: execution.ID.PG(), ActionIndex: int32(index), Result: encoded,
		})
		if err == nil && rows != 1 {
			return errors.New("automation action completion lost state")
		}
		return err
	})
}

func (repository *ExecutionPostgres) FailAction(ctx context.Context, execution Execution, index int, code string) error {
	return repository.withTenant(ctx, execution.WorkspaceID, execution.ID.String(), func(queries *dbgen.Queries) error {
		_, err := queries.FailAutomationAction(ctx, dbgen.FailAutomationActionParams{
			WorkspaceID: execution.WorkspaceID.PG(), ExecutionID: execution.ID.PG(), ActionIndex: int32(index), LastErrorCode: &code,
		})
		return err
	})
}

func (repository *ExecutionPostgres) Complete(ctx context.Context, execution Execution, results []map[string]any) error {
	encoded, err := json.Marshal(map[string]any{"actions": results})
	if err != nil {
		return err
	}
	return repository.withTenant(ctx, execution.WorkspaceID, execution.ID.String(), func(queries *dbgen.Queries) error {
		rows, err := queries.CompleteAutomationExecution(ctx, dbgen.CompleteAutomationExecutionParams{
			WorkspaceID: execution.WorkspaceID.PG(), ID: execution.ID.PG(), Result: encoded,
		})
		if err == nil && rows != 1 {
			return errors.New("automation execution completion lost state")
		}
		return err
	})
}

func (repository *ExecutionPostgres) Fail(
	ctx context.Context, execution Execution, code string, dead bool, results []map[string]any,
) error {
	encoded, err := json.Marshal(map[string]any{"actions": results})
	if err != nil {
		return err
	}
	return repository.withTenant(ctx, execution.WorkspaceID, execution.ID.String(), func(queries *dbgen.Queries) error {
		_, err := queries.FailAutomationExecution(ctx, dbgen.FailAutomationExecutionParams{
			WorkspaceID: execution.WorkspaceID.PG(), ID: execution.ID.PG(), Result: encoded,
			Dead: dead, ErrorCode: &code,
		})
		return err
	})
}

var _ DispatchRepository = (*dispatchPostgres)(nil)
var _ ExecutionRepository = (*ExecutionPostgres)(nil)
