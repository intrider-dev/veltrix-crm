package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/assignment"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) ListTaskAssignments(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, activityID ids.UUID,
) (assignment.Set, error) {
	activity, err := service.Get(ctx, workspace, workspaceID, activityID)
	if err != nil {
		return assignment.Set{}, err
	}
	if activity.Type != "task" {
		return assignment.Set{}, errx.ErrNotFound
	}
	rows, err := workspace.Queries.ListActivityAssignments(ctx, dbgen.ListActivityAssignmentsParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
	})
	if err != nil {
		return assignment.Set{}, fmt.Errorf("list task assignments: %w", err)
	}
	items := make([]assignment.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, assignment.NewItem(row.ID, row.AssignmentKind, row.UserID, row.DepartmentID,
			row.IsPrimary, row.UserName, row.DepartmentName, row.CreatedAt.Time))
	}
	return assignment.Set{Items: items, Version: activity.Version}, nil
}

func (service *Service) ReplaceTaskAssignments(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	activityID ids.UUID, version int64, items []assignment.Input,
) (assignment.Set, error) {
	if err := assignment.Validate(items); err != nil {
		return assignment.Set{}, err
	}
	activity, err := service.Get(ctx, workspace, metadata.WorkspaceID, activityID)
	if err != nil {
		return assignment.Set{}, err
	}
	if activity.Type != "task" {
		return assignment.Set{}, errx.ErrNotFound
	}
	lockedVersion, err := workspace.Queries.LockTaskForAssignmentUpdate(ctx, dbgen.LockTaskForAssignmentUpdateParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return assignment.Set{}, errx.ErrForbidden
	}
	if err != nil {
		return assignment.Set{}, fmt.Errorf("lock task assignment update: %w", err)
	}
	if lockedVersion != version {
		return assignment.Set{}, errx.ErrVersionConflict
	}
	if err := workspace.Queries.DeleteActivityAssignments(ctx, dbgen.DeleteActivityAssignmentsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
	}); err != nil {
		return assignment.Set{}, fmt.Errorf("delete task assignments: %w", err)
	}
	var primaryAssignee *ids.UUID
	for _, item := range items {
		id, err := ids.NewV7()
		if err != nil {
			return assignment.Set{}, err
		}
		userID, departmentID := taskAssignmentSubjectIDs(item)
		if item.IsPrimary && item.SubjectType == "user" {
			value := item.SubjectID
			primaryAssignee = &value
		}
		if err := workspace.Queries.CreateActivityAssignment(ctx, dbgen.CreateActivityAssignmentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: id.PG(), ActivityID: activityID.PG(),
			AssignmentKind: item.Kind, UserID: userID, DepartmentID: departmentID,
			IsPrimary: item.IsPrimary, CreatedBy: metadata.ActorID.PG(),
		}); err != nil {
			return assignment.Set{}, mapActivityAssignmentConstraint(err)
		}
	}
	newVersion, err := workspace.Queries.BumpActivityAssignmentsVersion(ctx, dbgen.BumpActivityAssignmentsVersionParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: activityID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return assignment.Set{}, errx.ErrVersionConflict
	}
	if err != nil {
		return assignment.Set{}, fmt.Errorf("bump task assignment version: %w", err)
	}
	if err := service.recordActivityMutation(ctx, workspace, metadata, events.Mutation{
		Action: "task.assignments_replaced", EventType: "activities.task.assignments_replaced",
		AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"assignmentCount": len(items)},
		Payload: map[string]any{"activityId": activityID.String(), "version": newVersion},
	}, activity.VisibilityScope, optionalUUID(activity.ScopeDepartmentID), optionalUUID(activity.ScopeUserID),
		optionalUUID(primaryAssignee), activity.CreatedBy.PG()); err != nil {
		return assignment.Set{}, err
	}
	result, err := service.ListTaskAssignments(ctx, workspace, metadata.WorkspaceID, activityID)
	result.Version = newVersion
	return result, err
}

func taskAssignmentSubjectIDs(item assignment.Input) (pgtype.UUID, pgtype.UUID) {
	if item.SubjectType == "user" {
		return item.SubjectID.PG(), pgtype.UUID{}
	}
	return pgtype.UUID{}, item.SubjectID.PG()
}

func mapActivityAssignmentConstraint(err error) error {
	var pgError interface{ SQLState() string }
	if errors.As(err, &pgError) {
		switch pgError.SQLState() {
		case "23503":
			return validationError("/assignments", "validation.reference.invalid")
		case "23505":
			return validationError("/assignments", "validation.duplicate")
		case "23514":
			return validationError("/assignments", "validation.constraint")
		}
	}
	return fmt.Errorf("task assignment constraint: %w", err)
}
