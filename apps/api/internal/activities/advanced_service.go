package activities

import (
	"context"
	"encoding/json"
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
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/pagination"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) CreateAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input AdvancedInput,
) (Activity, error) {
	row, err := service.Create(ctx, workspace, metadata, input)
	if err != nil {
		return Activity{}, err
	}
	return activityFromCreateAdvanced(row), nil
}

func (service *Service) Get(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, activityID ids.UUID,
) (Activity, error) {
	row, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Activity{}, errx.ErrNotFound
	}
	if err != nil {
		return Activity{}, fmt.Errorf("get activity: %w", err)
	}
	return activityFromGet(row), nil
}

func (service *Service) Update(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	activityID ids.UUID,
	expectedVersion int64,
	input AdvancedInput,
) (Activity, error) {
	input, err := validateActivityInput(input)
	if err != nil {
		return Activity{}, err
	}
	if err := service.validateReferences(ctx, workspace, metadata.WorkspaceID, input); err != nil {
		return Activity{}, err
	}
	row, err := workspace.Queries.UpdateActivity(ctx, dbgen.UpdateActivityParams{
		ActivityType: input.Type, Title: input.Title, Body: input.Body,
		RelatedType: input.RelatedType, RelatedID: optionalUUID(input.RelatedID),
		AssigneeUserID: optionalUUID(input.AssigneeID), Status: input.Status,
		Priority: input.Priority, DueAt: optionalTime(input.DueAt),
		OccurredAt: pgtype.Timestamptz{Time: input.OccurredAt, Valid: true},
		EndsAt:     optionalTime(input.EndsAt), Location: input.Location,
		RecurrenceRule: input.RecurrenceRule, VisibilityScope: input.VisibilityScope,
		ScopeDepartmentID: optionalUUID(input.ScopeDepartmentID), ScopeUserID: optionalUUID(input.ScopeUserID),
		WorkspaceID: metadata.WorkspaceID.PG(),
		ActivityID:  activityID.PG(), ActorUserID: workspace.Membership.UserID,
		ActorRole: workspace.Membership.Role, ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Activity{}, errx.ErrVersionConflict
	}
	if err != nil {
		return Activity{}, fmt.Errorf("update activity: %w", err)
	}
	if input.Type == "note" && input.VisibilityScope == "workspace" {
		if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "note", EntityID: activityID.PG(),
			Title: row.Title, Subtitle: row.RelatedType,
			SearchableText: strings.Join(nonEmpty(row.Title, pointerValue(row.Body)), " "),
			RankBoost:      0.7, Version: row.Version,
		}); err != nil {
			return Activity{}, fmt.Errorf("update note search document: %w", err)
		}
	} else if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "note", EntityID: activityID.PG(),
	}); err != nil {
		return Activity{}, fmt.Errorf("remove note search document: %w", err)
	}
	if input.VisibilityScope == "workspace" {
		if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
			return Activity{}, fmt.Errorf("refresh dashboard: %w", err)
		}
	}
	if err := service.recordActivityMutation(ctx, workspace, metadata, events.Mutation{
		Action: "activity.updated", EventType: "activities.activity.updated",
		AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"type": input.Type, "status": input.Status, "visibilityScope": input.VisibilityScope},
		Payload: map[string]any{"activityId": activityID.String(), "type": input.Type, "version": row.Version},
	}, row.VisibilityScope, row.ScopeDepartmentID, row.ScopeUserID, row.AssigneeUserID, row.CreatedBy); err != nil {
		return Activity{}, err
	}
	return activityFromUpdate(row), nil
}

func (service *Service) Delete(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	activityID ids.UUID,
	expectedVersion int64,
) error {
	current, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get activity before delete: %w", err)
	}
	updated, err := workspace.Queries.SoftDeleteActivity(ctx, dbgen.SoftDeleteActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return fmt.Errorf("delete activity: %w", err)
	}
	if updated != 1 {
		return errx.ErrVersionConflict
	}
	if current.ActivityType == "note" {
		if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "note", EntityID: activityID.PG(),
		}); err != nil {
			return fmt.Errorf("remove deleted note from search: %w", err)
		}
	}
	if current.VisibilityScope == "workspace" {
		if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
			return fmt.Errorf("refresh dashboard: %w", err)
		}
	}
	return service.recordActivityMutation(ctx, workspace, metadata, events.Mutation{
		Action: "activity.deleted", EventType: "activities.activity.deleted",
		AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"type": current.ActivityType, "visibilityScope": current.VisibilityScope},
		Payload: map[string]any{"activityId": activityID.String(), "type": current.ActivityType},
	}, current.VisibilityScope, current.ScopeDepartmentID, current.ScopeUserID,
		current.AssigneeUserID, current.CreatedBy)
}

func (service *Service) Timeline(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	entityID ids.UUID,
	cursor string,
	limit int,
) (ActivityPage, error) {
	entityType = strings.TrimSpace(entityType)
	if !oneOf(entityType, "contact", "company", "deal") {
		return ActivityPage{}, validationError("/query/entityType", "validation.enum")
	}
	exists, err := service.resolver.Exists(ctx, workspace, workspaceID, entityType, entityID)
	if err != nil {
		return ActivityPage{}, err
	}
	if !exists {
		return ActivityPage{}, errx.ErrNotFound
	}
	limit = boundedLimit(limit)
	filter := "timeline=" + entityType + ":" + entityID.String()
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return ActivityPage{}, validationError("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListEntityTimeline(ctx, dbgen.ListEntityTimelineParams{
		WorkspaceID: workspaceID.PG(), EntityType: &entityType, EntityID: entityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
		CursorOccurredAt:  timestamp(cursorTime), CursorID: cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return ActivityPage{}, fmt.Errorf("list entity timeline: %w", err)
	}
	items := make([]Activity, 0, min(len(rows), limit))
	for index, row := range rows {
		if index == limit {
			break
		}
		items = append(items, activityFromTimeline(row))
	}
	page := ActivityPage{Items: items}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.OccurredAt.Time, lastID, filter)
		if err != nil {
			return ActivityPage{}, err
		}
	}
	return page, nil
}

func (service *Service) Feed(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	activityType, status string,
	assigneeID *ids.UUID,
	cursor string,
	limit int,
) (ActivityPage, error) {
	activityType = strings.TrimSpace(activityType)
	status = strings.TrimSpace(status)
	if activityType != "" && !oneOf(activityType, "task", "call", "meeting", "note") {
		return ActivityPage{}, validationError("/query/type", "validation.enum")
	}
	if status != "" && !oneOf(status, "open", "completed", "cancelled") {
		return ActivityPage{}, validationError("/query/status", "validation.enum")
	}
	limit = boundedLimit(limit)
	filter := "feed=" + activityType + ":" + status + ":" + uuidValue(assigneeID)
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return ActivityPage{}, validationError("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListActivityFeed(ctx, dbgen.ListActivityFeedParams{
		WorkspaceID: workspaceID.PG(), ActivityType: activityType, StatusFilter: status,
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
		AssigneeUserID:    optionalUUID(assigneeID), CursorOccurredAt: timestamp(cursorTime),
		CursorID: cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return ActivityPage{}, fmt.Errorf("list activity feed: %w", err)
	}
	items := make([]Activity, 0, min(len(rows), limit))
	for index, row := range rows {
		if index == limit {
			break
		}
		items = append(items, activityFromFeed(row))
	}
	page := ActivityPage{Items: items}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.OccurredAt.Time, lastID, filter)
		if err != nil {
			return ActivityPage{}, err
		}
	}
	return page, nil
}

func (service *Service) Calendar(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	start, end time.Time,
) ([]Activity, error) {
	if err := validateCalendarRange(start, end); err != nil {
		return nil, err
	}
	rows, err := workspace.Queries.ListCalendarActivities(ctx, dbgen.ListCalendarActivitiesParams{
		WorkspaceID: workspaceID.PG(), RangeStart: timestamp(start.UTC()),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
		RangeEnd:          timestamp(end.UTC()), ResultLimit: 5000,
	})
	if err != nil {
		return nil, fmt.Errorf("list calendar activities: %w", err)
	}
	items := make([]Activity, 0, len(rows))
	for _, row := range rows {
		items = append(items, activityFromCalendar(row))
	}
	return items, nil
}

func (service *Service) CreateComment(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	activityID ids.UUID,
	actorDisplayName string,
	input CommentInput,
	notify MentionNotifier,
) (Comment, error) {
	body := strings.TrimSpace(input.Body)
	if len(body) < 1 || len(body) > 10_000 {
		return Comment{}, validationError("/body", "validation.length")
	}
	activity, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, errx.ErrNotFound
	}
	if err != nil {
		return Comment{}, fmt.Errorf("get commented activity: %w", err)
	}
	mentions, pgMentions, err := validateMentionIDs(ctx, workspace, metadata.WorkspaceID, input.MentionedUserIDs)
	if err != nil {
		return Comment{}, err
	}
	if activity.VisibilityScope != "workspace" {
		audience, audienceErr := service.resolveActivityAudience(ctx, workspace, metadata.WorkspaceID,
			activityID, activity.VisibilityScope, activity.ScopeDepartmentID, activity.ScopeUserID,
			activity.AssigneeUserID, activity.CreatedBy)
		if audienceErr != nil {
			return Comment{}, audienceErr
		}
		allowed := make(map[ids.UUID]struct{}, len(audience))
		for _, userID := range audience {
			allowed[userID] = struct{}{}
		}
		for _, mentionedID := range mentions {
			if _, ok := allowed[mentionedID]; !ok {
				return Comment{}, validationError("/mentionedUserIds", "validation.reference.invalid")
			}
		}
	}
	commentID, err := ids.NewV7()
	if err != nil {
		return Comment{}, err
	}
	row, err := workspace.Queries.CreateActivityComment(ctx, dbgen.CreateActivityCommentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), CommentID: commentID.PG(),
		ActivityID: activityID.PG(), AuthorUserID: metadata.ActorID.PG(),
		Body: body, MentionedUserIds: pgMentions,
	})
	if err != nil {
		return Comment{}, fmt.Errorf("create activity comment: %w", err)
	}
	if notify != nil {
		params, marshalErr := json.Marshal(map[string]any{
			"actorName": actorDisplayName, "title": activity.Title,
		})
		if marshalErr != nil {
			return Comment{}, marshalErr
		}
		for _, recipientID := range mentions {
			if recipientID == metadata.ActorID {
				continue
			}
			if err := notify(ctx, MentionNotification{
				RecipientUserID: recipientID, MessageKey: "notifications.comment.mention",
				MessageParams: params, ActivityID: activityID,
			}); err != nil {
				return Comment{}, fmt.Errorf("create mention notification: %w", err)
			}
		}
	}
	if err := service.recordMutationForActivity(ctx, workspace, metadata, activityID, events.Mutation{
		Action: "activity.comment.created", EventType: "activities.comment.created",
		AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"commentId": commentID.String(), "mentionCount": len(mentions)},
		Payload: map[string]any{"activityId": activityID.String(), "commentId": commentID.String()},
	}); err != nil {
		return Comment{}, err
	}
	return commentFromCreate(row), nil
}

func (service *Service) ListComments(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, activityID ids.UUID,
	cursor string,
	limit int,
) (CommentPage, error) {
	if _, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return CommentPage{}, errx.ErrNotFound
	} else if err != nil {
		return CommentPage{}, fmt.Errorf("get activity before comments: %w", err)
	}
	limit = boundedLimit(limit)
	filter := "comments=" + activityID.String()
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return CommentPage{}, validationError("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListActivityComments(ctx, dbgen.ListActivityCommentsParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
		CursorCreatedAt: timestamp(cursorTime), CursorID: cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return CommentPage{}, fmt.Errorf("list activity comments: %w", err)
	}
	items := make([]Comment, 0, min(len(rows), limit))
	for index, row := range rows {
		if index == limit {
			break
		}
		items = append(items, commentFromList(row))
	}
	page := CommentPage{Items: items}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.CreatedAt.Time, lastID, filter)
		if err != nil {
			return CommentPage{}, err
		}
	}
	return page, nil
}

func (service *Service) UpdateComment(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	commentID ids.UUID,
	expectedVersion int64,
	input CommentInput,
) (Comment, error) {
	body := strings.TrimSpace(input.Body)
	if len(body) < 1 || len(body) > 10_000 {
		return Comment{}, validationError("/body", "validation.length")
	}
	_, pgMentions, err := validateMentionIDs(ctx, workspace, metadata.WorkspaceID, input.MentionedUserIDs)
	if err != nil {
		return Comment{}, err
	}
	row, err := workspace.Queries.UpdateActivityComment(ctx, dbgen.UpdateActivityCommentParams{
		Body: body, MentionedUserIds: pgMentions, WorkspaceID: metadata.WorkspaceID.PG(),
		CommentID: commentID.PG(), AuthorUserID: metadata.ActorID.PG(), ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, errx.ErrVersionConflict
	}
	if err != nil {
		return Comment{}, fmt.Errorf("update activity comment: %w", err)
	}
	if err := service.recordMutationForActivity(ctx, workspace, metadata, mustID(row.ActivityID), events.Mutation{
		Action: "activity.comment.updated", EventType: "activities.comment.updated",
		AggregateType: "activity", AggregateID: mustID(row.ActivityID),
		Summary: map[string]any{"commentId": commentID.String()},
		Payload: map[string]any{"activityId": mustID(row.ActivityID).String(), "commentId": commentID.String(), "version": row.Version},
	}); err != nil {
		return Comment{}, err
	}
	return commentFromUpdate(row), nil
}

func (service *Service) DeleteComment(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	commentID ids.UUID,
	expectedVersion int64,
) error {
	rows, err := workspace.Queries.SoftDeleteActivityComment(ctx, dbgen.SoftDeleteActivityCommentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), CommentID: commentID.PG(),
		AuthorUserID: metadata.ActorID.PG(), ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return fmt.Errorf("delete activity comment: %w", err)
	}
	if rows != 1 {
		return errx.ErrVersionConflict
	}
	return nil
}

func (service *Service) CreateReminder(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	activityID ids.UUID,
	input ReminderInput,
) (Reminder, error) {
	input.Channel = strings.TrimSpace(input.Channel)
	if input.Channel == "" {
		input.Channel = "in_app"
	}
	if !oneOf(input.Channel, "in_app", "email", "both") {
		return Reminder{}, validationError("/channel", "validation.enum")
	}
	input.RemindAt = input.RemindAt.UTC()
	if input.RemindAt.IsZero() || input.RemindAt.After(time.Now().UTC().AddDate(5, 0, 0)) {
		return Reminder{}, validationError("/remindAt", "validation.range")
	}
	activity, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Reminder{}, errx.ErrNotFound
	} else if err != nil {
		return Reminder{}, fmt.Errorf("get reminder activity: %w", err)
	}
	if err := validateActiveUser(ctx, workspace, metadata.WorkspaceID, input.RecipientUserID, "/recipientUserId"); err != nil {
		return Reminder{}, err
	}
	if activity.VisibilityScope != "workspace" {
		audience, audienceErr := service.resolveActivityAudience(ctx, workspace, metadata.WorkspaceID,
			activityID, activity.VisibilityScope, activity.ScopeDepartmentID, activity.ScopeUserID,
			activity.AssigneeUserID, activity.CreatedBy)
		if audienceErr != nil {
			return Reminder{}, audienceErr
		}
		allowed := false
		for _, userID := range audience {
			if userID == input.RecipientUserID {
				allowed = true
				break
			}
		}
		if !allowed {
			return Reminder{}, validationError("/recipientUserId", "validation.reference.invalid")
		}
	}
	reminderID, err := ids.NewV7()
	if err != nil {
		return Reminder{}, err
	}
	jobID, err := ids.NewV7()
	if err != nil {
		return Reminder{}, err
	}
	row, err := workspace.Queries.CreateActivityReminder(ctx, dbgen.CreateActivityReminderParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ReminderID: reminderID.PG(),
		ActivityID: activityID.PG(), RecipientUserID: input.RecipientUserID.PG(),
		RemindAt: timestamp(input.RemindAt), Channel: input.Channel,
	})
	if err != nil {
		return Reminder{}, fmt.Errorf("create activity reminder: %w", err)
	}
	if err := workspace.Queries.EnqueueActivityReminderJob(ctx, dbgen.EnqueueActivityReminderJobParams{
		WorkspaceID: metadata.WorkspaceID.PG(), JobID: jobID.PG(), ReminderID: reminderID.String(),
		RecipientUserID: input.RecipientUserID.String(), RemindAt: timestamp(input.RemindAt),
	}); err != nil {
		return Reminder{}, fmt.Errorf("enqueue activity reminder: %w", err)
	}
	if err := service.recordMutationForActivity(ctx, workspace, metadata, activityID, events.Mutation{
		Action: "activity.reminder.created", EventType: "activities.reminder.created",
		AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"reminderId": reminderID.String(), "channel": input.Channel},
		Payload: map[string]any{"activityId": activityID.String(), "reminderId": reminderID.String()},
	}); err != nil {
		return Reminder{}, err
	}
	return reminderFromCreate(row), nil
}

func (service *Service) ListReminders(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, activityID ids.UUID,
) ([]Reminder, error) {
	if _, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, errx.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get activity before reminders: %w", err)
	}
	rows, err := workspace.Queries.ListActivityReminders(ctx, dbgen.ListActivityRemindersParams{
		WorkspaceID: workspaceID.PG(), ActivityID: activityID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("list activity reminders: %w", err)
	}
	result := make([]Reminder, 0, len(rows))
	for _, row := range rows {
		result = append(result, reminderFromList(row))
	}
	return result, nil
}

func (service *Service) UpdateReminder(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	reminderID ids.UUID,
	expectedVersion int64,
	input ReminderInput,
) (Reminder, error) {
	input.Channel = strings.TrimSpace(input.Channel)
	if !oneOf(input.Channel, "in_app", "email", "both") {
		return Reminder{}, validationError("/channel", "validation.enum")
	}
	input.RemindAt = input.RemindAt.UTC()
	if input.RemindAt.IsZero() || input.RemindAt.After(time.Now().UTC().AddDate(5, 0, 0)) {
		return Reminder{}, validationError("/remindAt", "validation.range")
	}
	row, err := workspace.Queries.UpdateActivityReminder(ctx, dbgen.UpdateActivityReminderParams{
		RemindAt: timestamp(input.RemindAt), Channel: input.Channel,
		WorkspaceID: metadata.WorkspaceID.PG(), ReminderID: reminderID.PG(), ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Reminder{}, errx.ErrVersionConflict
	}
	if err != nil {
		return Reminder{}, fmt.Errorf("update activity reminder: %w", err)
	}
	jobID, err := ids.NewV7()
	if err != nil {
		return Reminder{}, err
	}
	if err := workspace.Queries.EnqueueActivityReminderJob(ctx, dbgen.EnqueueActivityReminderJobParams{
		WorkspaceID: metadata.WorkspaceID.PG(), JobID: jobID.PG(), ReminderID: reminderID.String(),
		RecipientUserID: mustID(row.RecipientUserID).String(), RemindAt: timestamp(input.RemindAt),
	}); err != nil {
		return Reminder{}, fmt.Errorf("reschedule activity reminder: %w", err)
	}
	activityID := mustID(row.ActivityID)
	if err := service.recordMutationForActivity(ctx, workspace, metadata, activityID, events.Mutation{
		Action: "activity.reminder.updated", EventType: "activities.reminder.updated",
		AggregateType: "activity", AggregateID: activityID,
		Summary: map[string]any{"reminderId": reminderID.String(), "channel": input.Channel},
		Payload: map[string]any{"activityId": activityID.String(), "reminderId": reminderID.String(), "version": row.Version},
	}); err != nil {
		return Reminder{}, err
	}
	return reminderFromUpdate(row), nil
}

func (service *Service) recordMutationForActivity(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	activityID ids.UUID,
	mutation events.Mutation,
) error {
	activity, err := workspace.Queries.GetActivity(ctx, dbgen.GetActivityParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ActivityID: activityID.PG(),
		ActorUserID: workspace.Membership.UserID, ActorRole: workspace.Membership.Role,
		ActorMembershipID: workspace.Membership.ID,
	})
	if err != nil {
		return fmt.Errorf("load activity event audience: %w", err)
	}
	return service.recordActivityMutation(ctx, workspace, metadata, mutation,
		activity.VisibilityScope, activity.ScopeDepartmentID, activity.ScopeUserID,
		activity.AssigneeUserID, activity.CreatedBy)
}

func (service *Service) CancelReminder(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, reminderID ids.UUID,
	expectedVersion int64,
) error {
	rows, err := workspace.Queries.CancelActivityReminder(ctx, dbgen.CancelActivityReminderParams{
		WorkspaceID: workspaceID.PG(), ReminderID: reminderID.PG(), ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return fmt.Errorf("cancel activity reminder: %w", err)
	}
	if rows != 1 {
		return errx.ErrVersionConflict
	}
	return nil
}

func (service *Service) validateReferences(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, input AdvancedInput,
) error {
	if input.RelatedType != nil && input.RelatedID != nil {
		exists, err := service.resolver.Exists(ctx, workspace, workspaceID, *input.RelatedType, *input.RelatedID)
		if err != nil {
			return err
		}
		if !exists {
			return validationError("/relatedId", "validation.reference.invalid")
		}
	}
	if input.AssigneeID != nil {
		if err := validateActiveUser(ctx, workspace, workspaceID, *input.AssigneeID, "/assigneeId"); err != nil {
			return err
		}
	}
	if input.ScopeUserID != nil {
		if err := validateActiveUser(ctx, workspace, workspaceID, *input.ScopeUserID, "/scopeUserId"); err != nil {
			return err
		}
	}
	if input.ScopeDepartmentID != nil {
		_, err := workspace.Queries.GetTeam(ctx, dbgen.GetTeamParams{
			WorkspaceID: workspaceID.PG(), ID: input.ScopeDepartmentID.PG(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return validationError("/scopeDepartmentId", "validation.reference.invalid")
		}
		if err != nil {
			return fmt.Errorf("validate activity department: %w", err)
		}
	}
	return nil
}

func validateActiveUser(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, userID ids.UUID, pointer string,
) error {
	count, err := workspace.Queries.CountActiveMentionedUsers(ctx, dbgen.CountActiveMentionedUsersParams{
		MentionedUserIds: []pgtype.UUID{userID.PG()}, WorkspaceID: workspaceID.PG(),
	})
	if err != nil {
		return err
	}
	if count != 1 {
		return validationError(pointer, "validation.reference.invalid")
	}
	return nil
}

func validateMentionIDs(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	values []ids.UUID,
) ([]ids.UUID, []pgtype.UUID, error) {
	stringsToNormalize := make([]string, 0, len(values))
	for _, value := range values {
		stringsToNormalize = append(stringsToNormalize, value.String())
	}
	normalized, err := normalizeMentions(stringsToNormalize)
	if err != nil {
		return nil, nil, validationError("/mentionedUserIds", "validation.mentions.invalid")
	}
	mentions := make([]ids.UUID, 0, len(normalized))
	pgMentions := make([]pgtype.UUID, 0, len(normalized))
	for _, value := range normalized {
		mention, parseErr := ids.Parse(value)
		if parseErr != nil {
			return nil, nil, validationError("/mentionedUserIds", "validation.reference.invalid")
		}
		mentions = append(mentions, mention)
		pgMentions = append(pgMentions, mention.PG())
	}
	count, err := workspace.Queries.CountActiveMentionedUsers(ctx, dbgen.CountActiveMentionedUsersParams{
		MentionedUserIds: pgMentions, WorkspaceID: workspaceID.PG(),
	})
	if err != nil {
		return nil, nil, err
	}
	if count != int32(len(mentions)) {
		return nil, nil, validationError("/mentionedUserIds", "validation.reference.invalid")
	}
	return mentions, pgMentions, nil
}

func boundedLimit(value int) int {
	if value < 1 {
		return 50
	}
	if value > 100 {
		return 100
	}
	return value
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func uuidValue(value *ids.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func validationError(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
