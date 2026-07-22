package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Manager struct{}

func NewManager() *Manager { return &Manager{} }

func (manager *Manager) Create(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	spec RuleSpec,
) (Rule, error) {
	if err := ValidateRule(spec); err != nil {
		return Rule{}, err
	}
	id, err := ids.NewV7()
	if err != nil {
		return Rule{}, err
	}
	conditions, actions, err := encodeRuleParts(spec)
	if err != nil {
		return Rule{}, err
	}
	row, err := workspace.Queries.CreateAutomationRule(ctx, dbgen.CreateAutomationRuleParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: id.PG(), Name: strings.TrimSpace(spec.Name),
		TriggerType: string(spec.Trigger), EntityType: string(spec.EntityType), Conditions: conditions,
		Actions: actions, Enabled: spec.Enabled, RateLimitPerHour: int32(spec.RateLimitPerHour),
		CreatedBy: metadata.ActorID.PG(),
	})
	if err != nil {
		return Rule{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "automation_rule.created", EventType: "automation.rule.created",
		AggregateType: "automation_rule", AggregateID: id,
		Summary: map[string]any{"trigger": spec.Trigger, "entityType": spec.EntityType, "enabled": spec.Enabled},
		Payload: map[string]any{"ruleId": id.String(), "version": row.Version},
	}); err != nil {
		return Rule{}, err
	}
	return ruleFromDB(row)
}

func (manager *Manager) List(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, limit int) ([]Rule, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := workspace.Queries.ListAutomationRules(ctx, dbgen.ListAutomationRulesParams{
		WorkspaceID: workspaceID.PG(), Limit: int32(limit),
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

func (manager *Manager) Update(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	ruleID ids.UUID,
	version int64,
	spec RuleSpec,
) (Rule, error) {
	if err := ValidateRule(spec); err != nil {
		return Rule{}, err
	}
	conditions, actions, err := encodeRuleParts(spec)
	if err != nil {
		return Rule{}, err
	}
	row, err := workspace.Queries.UpdateAutomationRule(ctx, dbgen.UpdateAutomationRuleParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: ruleID.PG(), Name: strings.TrimSpace(spec.Name),
		TriggerType: string(spec.Trigger), EntityType: string(spec.EntityType), Conditions: conditions,
		Actions: actions, RateLimitPerHour: int32(spec.RateLimitPerHour), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, manager.versionError(ctx, workspace, metadata.WorkspaceID, ruleID)
	}
	if err != nil {
		return Rule{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "automation_rule.updated", EventType: "automation.rule.updated",
		AggregateType: "automation_rule", AggregateID: ruleID,
		Summary: map[string]any{"trigger": spec.Trigger, "entityType": spec.EntityType},
		Payload: map[string]any{"ruleId": ruleID.String(), "version": row.Version},
	}); err != nil {
		return Rule{}, err
	}
	return ruleFromDB(row)
}

func (manager *Manager) SetEnabled(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	ruleID ids.UUID,
	version int64,
	enabled bool,
) (Rule, error) {
	row, err := workspace.Queries.SetAutomationRuleEnabled(ctx, dbgen.SetAutomationRuleEnabledParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: ruleID.PG(), Enabled: enabled, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, manager.versionError(ctx, workspace, metadata.WorkspaceID, ruleID)
	}
	if err != nil {
		return Rule{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "automation_rule.enabled_changed", EventType: "automation.rule.enabled_changed",
		AggregateType: "automation_rule", AggregateID: ruleID,
		Summary: map[string]any{"enabled": enabled},
		Payload: map[string]any{"ruleId": ruleID.String(), "version": row.Version},
	}); err != nil {
		return Rule{}, err
	}
	return ruleFromDB(row)
}

func (manager *Manager) versionError(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, ruleID ids.UUID) error {
	_, err := workspace.Queries.GetAutomationRule(ctx, dbgen.GetAutomationRuleParams{WorkspaceID: workspaceID.PG(), ID: ruleID.PG()})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return err
	}
	return errx.ErrVersionConflict
}

func encodeRuleParts(spec RuleSpec) ([]byte, []byte, error) {
	conditions, err := json.Marshal(spec.Conditions)
	if err != nil {
		return nil, nil, fmt.Errorf("encode automation conditions: %w", err)
	}
	actions, err := json.Marshal(spec.Actions)
	if err != nil {
		return nil, nil, fmt.Errorf("encode automation actions: %w", err)
	}
	return conditions, actions, nil
}

func ruleFromDB(value any) (Rule, error) {
	var workspaceIDRaw, ruleIDRaw, createdByRaw pgtype.UUID
	var name, triggerType, entityType string
	var conditions, actions []byte
	var enabled bool
	var rateLimit int32
	var version int64
	var createdAt, updatedAt pgtype.Timestamptz
	switch row := value.(type) {
	case dbgen.CreateAutomationRuleRow:
		workspaceIDRaw, ruleIDRaw, createdByRaw = row.WorkspaceID, row.ID, row.CreatedBy
		name, triggerType, entityType = row.Name, row.TriggerType, row.EntityType
		conditions, actions, enabled, rateLimit, version = row.Conditions, row.Actions, row.Enabled, row.RateLimitPerHour, row.Version
		createdAt, updatedAt = row.CreatedAt, row.UpdatedAt
	case dbgen.GetAutomationRuleRow:
		workspaceIDRaw, ruleIDRaw, createdByRaw = row.WorkspaceID, row.ID, row.CreatedBy
		name, triggerType, entityType = row.Name, row.TriggerType, row.EntityType
		conditions, actions, enabled, rateLimit, version = row.Conditions, row.Actions, row.Enabled, row.RateLimitPerHour, row.Version
		createdAt, updatedAt = row.CreatedAt, row.UpdatedAt
	case dbgen.ListAutomationRulesRow:
		workspaceIDRaw, ruleIDRaw, createdByRaw = row.WorkspaceID, row.ID, row.CreatedBy
		name, triggerType, entityType = row.Name, row.TriggerType, row.EntityType
		conditions, actions, enabled, rateLimit, version = row.Conditions, row.Actions, row.Enabled, row.RateLimitPerHour, row.Version
		createdAt, updatedAt = row.CreatedAt, row.UpdatedAt
	case dbgen.ListEnabledAutomationRulesForTriggerRow:
		workspaceIDRaw, ruleIDRaw, createdByRaw = row.WorkspaceID, row.ID, row.CreatedBy
		name, triggerType, entityType = row.Name, row.TriggerType, row.EntityType
		conditions, actions, enabled, rateLimit, version = row.Conditions, row.Actions, row.Enabled, row.RateLimitPerHour, row.Version
		createdAt, updatedAt = row.CreatedAt, row.UpdatedAt
	case dbgen.UpdateAutomationRuleRow:
		workspaceIDRaw, ruleIDRaw, createdByRaw = row.WorkspaceID, row.ID, row.CreatedBy
		name, triggerType, entityType = row.Name, row.TriggerType, row.EntityType
		conditions, actions, enabled, rateLimit, version = row.Conditions, row.Actions, row.Enabled, row.RateLimitPerHour, row.Version
		createdAt, updatedAt = row.CreatedAt, row.UpdatedAt
	case dbgen.SetAutomationRuleEnabledRow:
		workspaceIDRaw, ruleIDRaw, createdByRaw = row.WorkspaceID, row.ID, row.CreatedBy
		name, triggerType, entityType = row.Name, row.TriggerType, row.EntityType
		conditions, actions, enabled, rateLimit, version = row.Conditions, row.Actions, row.Enabled, row.RateLimitPerHour, row.Version
		createdAt, updatedAt = row.CreatedAt, row.UpdatedAt
	default:
		return Rule{}, fmt.Errorf("unsupported automation rule row %T", value)
	}
	workspaceID, workspaceOK := ids.FromPG(workspaceIDRaw)
	ruleID, ruleOK := ids.FromPG(ruleIDRaw)
	createdBy, creatorOK := ids.FromPG(createdByRaw)
	if !workspaceOK || !ruleOK || !creatorOK {
		return Rule{}, errors.New("automation rule contains an invalid identifier")
	}
	spec := RuleSpec{
		Name: name, Trigger: TriggerType(triggerType), EntityType: EntityType(entityType),
		Enabled: enabled, RateLimitPerHour: int(rateLimit),
	}
	if err := json.Unmarshal(conditions, &spec.Conditions); err != nil {
		return Rule{}, fmt.Errorf("decode automation conditions: %w", err)
	}
	if err := json.Unmarshal(actions, &spec.Actions); err != nil {
		return Rule{}, fmt.Errorf("decode automation actions: %w", err)
	}
	if err := ValidateRule(spec); err != nil {
		return Rule{}, fmt.Errorf("persisted automation rule is invalid: %w", err)
	}
	return Rule{
		WorkspaceID: workspaceID, ID: ruleID, CreatedBy: createdBy, Spec: spec,
		Version: version, CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC(),
	}, nil
}
