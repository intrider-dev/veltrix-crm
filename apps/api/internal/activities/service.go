package activities

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type TargetResolver interface {
	Exists(context.Context, *tenancy.WorkspaceTx, ids.UUID, string, ids.UUID) (bool, error)
}

type Service struct {
	resolver TargetResolver
}

func NewService(resolver TargetResolver) *Service { return &Service{resolver: resolver} }

func (service *Service) List(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	entityID *ids.UUID,
	limit int,
) ([]dbgen.ListActivitiesRow, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := workspace.Queries.ListActivities(ctx, dbgen.ListActivitiesParams{
		WorkspaceID: workspaceID.PG(), ActorUserID: workspace.Membership.UserID,
		ActorRole: workspace.Membership.Role, ActorMembershipID: workspace.Membership.ID,
		EntityType: entityType, EntityID: optionalUUID(entityID), PageLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	return rows, nil
}

func (service *Service) Create(ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, input Input) (dbgen.CreateActivityRow, error) {
	input, err := validateActivityInput(input)
	if err != nil {
		return dbgen.CreateActivityRow{}, err
	}
	if input.Status != "open" {
		return dbgen.CreateActivityRow{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/status", Code: "validation.create_status.invalid",
		}}}
	}
	if err := service.validateReferences(ctx, workspace, metadata.WorkspaceID, input); err != nil {
		return dbgen.CreateActivityRow{}, err
	}
	activityID, err := ids.NewV7()
	if err != nil {
		return dbgen.CreateActivityRow{}, err
	}
	row, err := workspace.Queries.CreateActivity(ctx, dbgen.CreateActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: activityID.PG(), ActivityType: input.Type, Title: input.Title,
		Body: trimPointer(input.Body), RelatedType: input.RelatedType, RelatedID: optionalUUID(input.RelatedID),
		AssigneeUserID: optionalUUID(input.AssigneeID), Priority: input.Priority, DueAt: optionalTime(input.DueAt),
		OccurredAt: pgtype.Timestamptz{Time: input.OccurredAt, Valid: true}, EndsAt: optionalTime(input.EndsAt),
		Location: input.Location, RecurrenceRule: input.RecurrenceRule,
		VisibilityScope: input.VisibilityScope, ScopeDepartmentID: optionalUUID(input.ScopeDepartmentID),
		ScopeUserID: optionalUUID(input.ScopeUserID), CreatedBy: metadata.ActorID.PG(),
	})
	if err != nil {
		return dbgen.CreateActivityRow{}, fmt.Errorf("create activity: %w", err)
	}
	if input.Type == "note" && input.VisibilityScope == "workspace" {
		subtitle := input.RelatedType
		if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "note", EntityID: activityID.PG(), Title: row.Title,
			Subtitle: subtitle, SearchableText: strings.Join(nonEmpty(row.Title, pointerValue(row.Body)), " "), RankBoost: 0.7, Version: row.Version,
		}); err != nil {
			return dbgen.CreateActivityRow{}, err
		}
	}
	if input.VisibilityScope == "workspace" {
		if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
			return dbgen.CreateActivityRow{}, err
		}
	}
	mutation := events.Mutation{
		Action: "activity.created", EventType: "activities.activity.created", AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"type": input.Type, "relatedType": input.RelatedType, "visibilityScope": input.VisibilityScope},
		Payload: map[string]any{"activityId": activityID.String(), "type": input.Type, "version": row.Version},
	}
	if err := service.recordActivityMutation(ctx, workspace, metadata, mutation, row.VisibilityScope,
		row.ScopeDepartmentID, row.ScopeUserID, row.AssigneeUserID, row.CreatedBy); err != nil {
		return dbgen.CreateActivityRow{}, err
	}
	return row, nil
}

func (service *Service) Complete(ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, activityID ids.UUID, version int64) (dbgen.CompleteActivityRow, error) {
	row, err := workspace.Queries.CompleteActivity(ctx, dbgen.CompleteActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: activityID.PG(), Version: version,
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, getErr := service.Get(ctx, workspace, metadata.WorkspaceID, activityID)
		if getErr != nil {
			return dbgen.CompleteActivityRow{}, getErr
		}
		if current.Version != version || current.Status != "open" {
			return dbgen.CompleteActivityRow{}, errx.ErrVersionConflict
		}
		return dbgen.CompleteActivityRow{}, errx.ErrForbidden
	}
	if err != nil {
		return dbgen.CompleteActivityRow{}, err
	}
	if row.VisibilityScope == "workspace" {
		if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
			return dbgen.CompleteActivityRow{}, err
		}
	}
	if err := service.recordActivityMutation(ctx, workspace, metadata, events.Mutation{
		Action: "activity.completed", EventType: "activities.activity.completed", AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"status": "completed"}, Payload: map[string]any{"activityId": activityID.String(), "version": row.Version},
	}, row.VisibilityScope, row.ScopeDepartmentID, row.ScopeUserID, row.AssigneeUserID, row.CreatedBy); err != nil {
		return dbgen.CompleteActivityRow{}, err
	}
	return row, nil
}

func (service *Service) recordActivityMutation(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	mutation events.Mutation,
	visibilityScope string,
	scopeDepartmentID, scopeUserID, assigneeUserID, createdBy pgtype.UUID,
) error {
	if visibilityScope == "workspace" {
		return events.Record(ctx, workspace.Queries, metadata, mutation)
	}
	recipients, err := service.resolveActivityAudience(ctx, workspace, metadata.WorkspaceID,
		mutation.AggregateID, visibilityScope, scopeDepartmentID, scopeUserID, assigneeUserID, createdBy)
	if err != nil {
		return err
	}
	mutation.EventType = "activities.private." + strings.TrimPrefix(mutation.EventType, "activities.")
	return events.RecordTargeted(ctx, workspace.Queries, metadata, mutation, recipients)
}

func (service *Service) resolveActivityAudience(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, activityID ids.UUID,
	visibilityScope string,
	scopeDepartmentID, scopeUserID, assigneeUserID, createdBy pgtype.UUID,
) ([]ids.UUID, error) {
	rows, err := workspace.Queries.ListActivityAudienceUserIDs(ctx, dbgen.ListActivityAudienceUserIDsParams{
		WorkspaceID: workspaceID.PG(), CreatedBy: createdBy,
		ScopeUserID: scopeUserID, AssigneeUserID: assigneeUserID,
		VisibilityScope: visibilityScope, ScopeDepartmentID: scopeDepartmentID,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve activity audience: %w", err)
	}
	recipients := make([]ids.UUID, 0, len(rows))
	seen := make(map[ids.UUID]struct{}, len(rows))
	for _, row := range rows {
		if recipient, valid := ids.FromPG(row); valid {
			recipients = append(recipients, recipient)
			seen[recipient] = struct{}{}
		}
	}
	assignmentRows, err := workspace.Queries.ListActivityAssignmentUserIDs(ctx, dbgen.ListActivityAssignmentUserIDsParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("resolve activity assignment audience: %w", err)
	}
	for _, row := range assignmentRows {
		if recipient, valid := ids.FromPG(row); valid {
			if _, exists := seen[recipient]; !exists {
				recipients = append(recipients, recipient)
				seen[recipient] = struct{}{}
			}
		}
	}
	return recipients, nil
}

func optionalUUID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func optionalTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
