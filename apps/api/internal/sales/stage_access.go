package sales

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const MaxStageRoleAccessRules = 256

type StageAccessAction string

const (
	StageAccessView  StageAccessAction = "view"
	StageAccessEnter StageAccessAction = "enter"
	StageAccessLeave StageAccessAction = "leave"
)

type StageRoleAccessInput struct {
	RoleID   ids.UUID
	CanView  bool
	CanEnter bool
	CanLeave bool
}

type StageRoleAccessRule struct {
	RoleID    ids.UUID
	RoleKey   string
	RoleName  string
	BaseRole  string
	CanView   bool
	CanEnter  bool
	CanLeave  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListLeadStageRoleAccess returns only explicit rules. An empty list means the
// stage inherits the resource-level permission already checked by WithWorkspace.
func (service *Service) ListLeadStageRoleAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, stageID ids.UUID,
) ([]StageRoleAccessRule, error) {
	if _, err := workspace.Queries.GetLeadStageForAccessConfig(ctx, dbgen.GetLeadStageForAccessConfigParams{
		WorkspaceID: workspaceID.PG(), StageID: stageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, errx.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get lead stage access target: %w", err)
	}
	rows, err := workspace.Queries.ListLeadStageRoleAccess(ctx, dbgen.ListLeadStageRoleAccessParams{
		WorkspaceID: workspaceID.PG(), StageID: stageID.PG(), ResultLimit: MaxStageRoleAccessRules + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list lead stage role access: %w", err)
	}
	return stageRoleAccessRules(rowsToStageAccess(rows))
}

// ReplaceLeadStageRoleAccess atomically replaces the full explicit rule set.
// The caller must run it inside a tenant transaction authorized with
// lead_stages.manage (or an equivalent administrative permission).
func (service *Service) ReplaceLeadStageRoleAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	stageID ids.UUID, rules []StageRoleAccessInput,
) ([]StageRoleAccessRule, error) {
	if err := validateStageRoleAccess(rules); err != nil {
		return nil, err
	}
	if _, err := workspace.Queries.LockLeadStageForAccessReplace(ctx, dbgen.LockLeadStageForAccessReplaceParams{
		WorkspaceID: metadata.WorkspaceID.PG(), StageID: stageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, errx.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock lead stage access target: %w", err)
	}
	if err := workspace.Queries.DeleteLeadStageRoleAccess(ctx, dbgen.DeleteLeadStageRoleAccessParams{
		WorkspaceID: metadata.WorkspaceID.PG(), StageID: stageID.PG(),
	}); err != nil {
		return nil, fmt.Errorf("delete lead stage role access: %w", err)
	}
	if err := insertLeadStageRoleAccess(ctx, workspace, metadata.WorkspaceID, stageID, rules); err != nil {
		return nil, err
	}
	if err := recordStageAccessMutation(ctx, workspace, metadata, "lead_stage", stageID, len(rules)); err != nil {
		return nil, err
	}
	return service.ListLeadStageRoleAccess(ctx, workspace, metadata.WorkspaceID, stageID)
}

// ListPipelineStageRoleAccess returns only explicit deal pipeline-stage rules.
func (service *Service) ListPipelineStageRoleAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, stageID ids.UUID,
) ([]StageRoleAccessRule, error) {
	if _, err := workspace.Queries.GetPipelineStageForAccessConfig(ctx, dbgen.GetPipelineStageForAccessConfigParams{
		WorkspaceID: workspaceID.PG(), StageID: stageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, errx.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get pipeline stage access target: %w", err)
	}
	rows, err := workspace.Queries.ListPipelineStageRoleAccess(ctx, dbgen.ListPipelineStageRoleAccessParams{
		WorkspaceID: workspaceID.PG(), StageID: stageID.PG(), ResultLimit: MaxStageRoleAccessRules + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list pipeline stage role access: %w", err)
	}
	return stageRoleAccessRules(pipelineRowsToStageAccess(rows))
}

// ReplacePipelineStageRoleAccess atomically replaces the full explicit rule set.
func (service *Service) ReplacePipelineStageRoleAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	stageID ids.UUID, rules []StageRoleAccessInput,
) ([]StageRoleAccessRule, error) {
	if err := validateStageRoleAccess(rules); err != nil {
		return nil, err
	}
	if _, err := workspace.Queries.LockPipelineStageForAccessReplace(ctx, dbgen.LockPipelineStageForAccessReplaceParams{
		WorkspaceID: metadata.WorkspaceID.PG(), StageID: stageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, errx.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock pipeline stage access target: %w", err)
	}
	if err := workspace.Queries.DeletePipelineStageRoleAccess(ctx, dbgen.DeletePipelineStageRoleAccessParams{
		WorkspaceID: metadata.WorkspaceID.PG(), StageID: stageID.PG(),
	}); err != nil {
		return nil, fmt.Errorf("delete pipeline stage role access: %w", err)
	}
	if err := insertPipelineStageRoleAccess(ctx, workspace, metadata.WorkspaceID, stageID, rules); err != nil {
		return nil, err
	}
	if err := recordStageAccessMutation(ctx, workspace, metadata, "pipeline_stage", stageID, len(rules)); err != nil {
		return nil, err
	}
	return service.ListPipelineStageRoleAccess(ctx, workspace, metadata.WorkspaceID, stageID)
}

func (service *Service) RequireLeadStageAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, stageID ids.UUID, action StageAccessAction,
) error {
	if !validStageAccessAction(action) {
		return fmt.Errorf("unsupported lead stage access action %q", action)
	}
	allowed, err := workspace.Queries.LeadStageAccessAllowed(ctx, dbgen.LeadStageAccessAllowedParams{
		WorkspaceID: workspaceID.PG(), StageID: stageID.PG(), AccessAction: string(action),
	})
	if err != nil {
		return fmt.Errorf("evaluate lead stage access: %w", err)
	}
	if !allowed {
		return errx.ErrForbidden
	}
	return nil
}

func (service *Service) RequirePipelineStageAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, stageID ids.UUID, action StageAccessAction,
) error {
	if !validStageAccessAction(action) {
		return fmt.Errorf("unsupported pipeline stage access action %q", action)
	}
	allowed, err := workspace.Queries.PipelineStageAccessAllowed(ctx, dbgen.PipelineStageAccessAllowedParams{
		WorkspaceID: workspaceID.PG(), StageID: stageID.PG(), AccessAction: string(action),
	})
	if err != nil {
		return fmt.Errorf("evaluate pipeline stage access: %w", err)
	}
	if !allowed {
		return errx.ErrForbidden
	}
	return nil
}

func (service *Service) RequireLeadStageTransition(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, fromStageID, toStageID ids.UUID,
) error {
	allowed, err := workspace.Queries.LeadStageTransitionAllowed(ctx, dbgen.LeadStageTransitionAllowedParams{
		WorkspaceID: workspaceID.PG(), FromStageID: fromStageID.PG(), ToStageID: toStageID.PG(),
	})
	if err != nil {
		return fmt.Errorf("evaluate lead stage transition: %w", err)
	}
	if !allowed {
		return errx.ErrForbidden
	}
	return nil
}

func (service *Service) RequirePipelineStageTransition(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, fromStageID, toStageID ids.UUID,
) error {
	allowed, err := workspace.Queries.PipelineStageTransitionAllowed(ctx, dbgen.PipelineStageTransitionAllowedParams{
		WorkspaceID: workspaceID.PG(), FromStageID: fromStageID.PG(), ToStageID: toStageID.PG(),
	})
	if err != nil {
		return fmt.Errorf("evaluate pipeline stage transition: %w", err)
	}
	if !allowed {
		return errx.ErrForbidden
	}
	return nil
}

type stageAccessRow struct {
	roleID                      pgtype.UUID
	roleKey, roleName, baseRole string
	canView, canEnter, canLeave bool
	createdAt, updatedAt        pgtype.Timestamptz
}

func rowsToStageAccess(rows []dbgen.ListLeadStageRoleAccessRow) []stageAccessRow {
	result := make([]stageAccessRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, stageAccessRow{row.RoleID, row.RoleKey, row.RoleName, row.BaseRole,
			row.CanView, row.CanEnter, row.CanLeave, row.CreatedAt, row.UpdatedAt})
	}
	return result
}

func pipelineRowsToStageAccess(rows []dbgen.ListPipelineStageRoleAccessRow) []stageAccessRow {
	result := make([]stageAccessRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, stageAccessRow{row.RoleID, row.RoleKey, row.RoleName, row.BaseRole,
			row.CanView, row.CanEnter, row.CanLeave, row.CreatedAt, row.UpdatedAt})
	}
	return result
}

func stageRoleAccessRules(rows []stageAccessRow) ([]StageRoleAccessRule, error) {
	if len(rows) > MaxStageRoleAccessRules {
		return nil, fmt.Errorf("stage role access rule count exceeds the bounded limit of %d", MaxStageRoleAccessRules)
	}
	result := make([]StageRoleAccessRule, 0, len(rows))
	for _, row := range rows {
		roleID, ok := ids.FromPG(row.roleID)
		if !ok {
			return nil, fmt.Errorf("stage role access contains an invalid role identifier")
		}
		result = append(result, StageRoleAccessRule{
			RoleID: roleID, RoleKey: row.roleKey, RoleName: row.roleName, BaseRole: row.baseRole,
			CanView: row.canView, CanEnter: row.canEnter, CanLeave: row.canLeave,
			CreatedAt: row.createdAt.Time.UTC(), UpdatedAt: row.updatedAt.Time.UTC(),
		})
	}
	return result, nil
}

func validateStageRoleAccess(rules []StageRoleAccessInput) error {
	if len(rules) > MaxStageRoleAccessRules {
		return validation("/rules", "validation.max_items")
	}
	seen := make(map[ids.UUID]struct{}, len(rules))
	for index, rule := range rules {
		if rule.RoleID == (ids.UUID{}) {
			return validation(fmt.Sprintf("/rules/%d/roleId", index), "validation.required")
		}
		if _, exists := seen[rule.RoleID]; exists {
			return validation(fmt.Sprintf("/rules/%d/roleId", index), "validation.duplicate")
		}
		seen[rule.RoleID] = struct{}{}
	}
	return nil
}

func insertLeadStageRoleAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, stageID ids.UUID,
	rules []StageRoleAccessInput,
) error {
	for _, rule := range rules {
		if err := workspace.Queries.CreateLeadStageRoleAccess(ctx, dbgen.CreateLeadStageRoleAccessParams{
			WorkspaceID: workspaceID.PG(), StageID: stageID.PG(), RoleID: rule.RoleID.PG(),
			CanView: rule.CanView, CanEnter: rule.CanEnter, CanLeave: rule.CanLeave,
		}); err != nil {
			return mapStageAccessConstraint(err)
		}
	}
	return nil
}

func insertPipelineStageRoleAccess(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, stageID ids.UUID,
	rules []StageRoleAccessInput,
) error {
	for _, rule := range rules {
		if err := workspace.Queries.CreatePipelineStageRoleAccess(ctx, dbgen.CreatePipelineStageRoleAccessParams{
			WorkspaceID: workspaceID.PG(), StageID: stageID.PG(), RoleID: rule.RoleID.PG(),
			CanView: rule.CanView, CanEnter: rule.CanEnter, CanLeave: rule.CanLeave,
		}); err != nil {
			return mapStageAccessConstraint(err)
		}
	}
	return nil
}

func mapStageAccessConstraint(err error) error {
	var postgresError interface{ SQLState() string }
	if errors.As(err, &postgresError) && postgresError.SQLState() == "23503" {
		return validation("/rules", "validation.reference.invalid")
	}
	return fmt.Errorf("replace stage role access: %w", err)
}

func recordStageAccessMutation(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	aggregateType string, stageID ids.UUID, ruleCount int,
) error {
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: aggregateType + ".access_replaced", EventType: "sales." + aggregateType + ".access_replaced",
		AggregateType: aggregateType, AggregateID: stageID,
		Summary: map[string]any{"ruleCount": ruleCount},
		Payload: map[string]any{"stageId": stageID.String()},
	})
}

func validStageAccessAction(action StageAccessAction) bool {
	return action == StageAccessView || action == StageAccessEnter || action == StageAccessLeave
}
