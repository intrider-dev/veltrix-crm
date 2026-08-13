package sales

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/assignment"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) ListLeadAssignments(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, leadID ids.UUID,
) (assignment.Set, error) {
	lead, err := service.GetLead(ctx, workspace, workspaceID, leadID)
	if err != nil {
		return assignment.Set{}, err
	}
	rows, err := workspace.Queries.ListLeadAssignments(ctx, dbgen.ListLeadAssignmentsParams{
		WorkspaceID: workspaceID.PG(), LeadID: leadID.PG(),
	})
	if err != nil {
		return assignment.Set{}, fmt.Errorf("list lead assignments: %w", err)
	}
	items := make([]assignment.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, assignment.NewItem(row.ID, row.AssignmentKind, row.UserID, row.DepartmentID,
			row.IsPrimary, row.UserName, row.DepartmentName, row.CreatedAt.Time))
	}
	return assignment.Set{Items: items, Version: lead.Version}, nil
}

func (service *Service) ReplaceLeadAssignments(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	leadID ids.UUID, version int64, items []assignment.Input,
) (assignment.Set, error) {
	if err := assignment.Validate(items); err != nil {
		return assignment.Set{}, err
	}
	if err := service.requireLeadVersion(ctx, workspace, metadata.WorkspaceID, leadID, version, false); err != nil {
		return assignment.Set{}, err
	}
	if err := workspace.Queries.DeleteLeadAssignments(ctx, dbgen.DeleteLeadAssignmentsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), LeadID: leadID.PG(),
	}); err != nil {
		return assignment.Set{}, fmt.Errorf("delete lead assignments: %w", err)
	}
	for _, item := range items {
		id, err := ids.NewV7()
		if err != nil {
			return assignment.Set{}, err
		}
		userID, departmentID := assignmentSubjectIDs(item)
		if err := workspace.Queries.CreateLeadAssignment(ctx, dbgen.CreateLeadAssignmentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: id.PG(), LeadID: leadID.PG(),
			AssignmentKind: item.Kind, UserID: userID, DepartmentID: departmentID,
			IsPrimary: item.IsPrimary, CreatedBy: metadata.ActorID.PG(),
		}); err != nil {
			return assignment.Set{}, mapConstraintError(err)
		}
	}
	newVersion, err := workspace.Queries.BumpLeadAssignmentsVersion(ctx, dbgen.BumpLeadAssignmentsVersionParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: leadID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return assignment.Set{}, service.classifyLeadMutation(ctx, workspace, metadata.WorkspaceID, leadID, version, false)
	}
	if err != nil {
		return assignment.Set{}, fmt.Errorf("bump lead assignment version: %w", err)
	}
	if err := recordAssignmentMutation(ctx, workspace, metadata, "lead", leadID, len(items), newVersion); err != nil {
		return assignment.Set{}, err
	}
	result, err := service.ListLeadAssignments(ctx, workspace, metadata.WorkspaceID, leadID)
	result.Version = newVersion
	return result, err
}

func (service *Service) ListDealAssignments(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, dealID ids.UUID,
) (assignment.Set, error) {
	deal, err := service.GetDealRecord(ctx, workspace, workspaceID, dealID)
	if err != nil {
		return assignment.Set{}, err
	}
	rows, err := workspace.Queries.ListDealAssignments(ctx, dbgen.ListDealAssignmentsParams{
		WorkspaceID: workspaceID.PG(), DealID: dealID.PG(),
	})
	if err != nil {
		return assignment.Set{}, fmt.Errorf("list deal assignments: %w", err)
	}
	items := make([]assignment.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, assignment.NewItem(row.ID, row.AssignmentKind, row.UserID, row.DepartmentID,
			row.IsPrimary, row.UserName, row.DepartmentName, row.CreatedAt.Time))
	}
	return assignment.Set{Items: items, Version: deal.Version}, nil
}

func (service *Service) ReplaceDealAssignments(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	dealID ids.UUID, version int64, items []assignment.Input,
) (assignment.Set, error) {
	if err := assignment.Validate(items); err != nil {
		return assignment.Set{}, err
	}
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, version, false); err != nil {
		return assignment.Set{}, err
	}
	if err := workspace.Queries.DeleteDealAssignments(ctx, dbgen.DeleteDealAssignmentsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(),
	}); err != nil {
		return assignment.Set{}, fmt.Errorf("delete deal assignments: %w", err)
	}
	for _, item := range items {
		id, err := ids.NewV7()
		if err != nil {
			return assignment.Set{}, err
		}
		userID, departmentID := assignmentSubjectIDs(item)
		if err := workspace.Queries.CreateDealAssignment(ctx, dbgen.CreateDealAssignmentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: id.PG(), DealID: dealID.PG(),
			AssignmentKind: item.Kind, UserID: userID, DepartmentID: departmentID,
			IsPrimary: item.IsPrimary, CreatedBy: metadata.ActorID.PG(),
		}); err != nil {
			return assignment.Set{}, mapConstraintError(err)
		}
	}
	newVersion, err := workspace.Queries.BumpDealAssignmentsVersion(ctx, dbgen.BumpDealAssignmentsVersionParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return assignment.Set{}, service.classifyDealMutation(ctx, workspace, metadata.WorkspaceID, dealID, version, false)
	}
	if err != nil {
		return assignment.Set{}, fmt.Errorf("bump deal assignment version: %w", err)
	}
	if err := recordAssignmentMutation(ctx, workspace, metadata, "deal", dealID, len(items), newVersion); err != nil {
		return assignment.Set{}, err
	}
	result, err := service.ListDealAssignments(ctx, workspace, metadata.WorkspaceID, dealID)
	result.Version = newVersion
	return result, err
}

func assignmentSubjectIDs(item assignment.Input) (pgtype.UUID, pgtype.UUID) {
	if item.SubjectType == "user" {
		return item.SubjectID.PG(), pgtype.UUID{}
	}
	return pgtype.UUID{}, item.SubjectID.PG()
}

func recordAssignmentMutation(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	entityType string, entityID ids.UUID, count int, version int64,
) error {
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: entityType + ".assignments_replaced", EventType: "sales." + entityType + ".assignments_replaced",
		AggregateType: entityType, AggregateID: entityID,
		Summary: map[string]any{"assignmentCount": count},
		Payload: map[string]any{entityType + "Id": entityID.String(), "version": version},
	})
}
