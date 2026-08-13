package activities

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type activityFields struct {
	id                pgtype.UUID
	activityType      string
	title             string
	body              *string
	relatedType       *string
	relatedID         pgtype.UUID
	assigneeUserID    pgtype.UUID
	status            string
	priority          string
	dueAt             pgtype.Timestamptz
	occurredAt        pgtype.Timestamptz
	endsAt            pgtype.Timestamptz
	location          *string
	recurrenceRule    *string
	visibilityScope   string
	scopeDepartmentID pgtype.UUID
	scopeUserID       pgtype.UUID
	completedAt       pgtype.Timestamptz
	createdBy         pgtype.UUID
	version           int64
	createdAt         pgtype.Timestamptz
	updatedAt         pgtype.Timestamptz
}

func mapActivity(value activityFields) Activity {
	return Activity{
		ID: mustID(value.id), Type: value.activityType, Title: value.title,
		Body: value.body, RelatedType: value.relatedType, RelatedID: optionalID(value.relatedID),
		AssigneeID: optionalID(value.assigneeUserID), Status: value.status, Priority: value.priority,
		DueAt: optionalTimestamp(value.dueAt), OccurredAt: value.occurredAt.Time.UTC(),
		EndsAt: optionalTimestamp(value.endsAt), Location: value.location,
		RecurrenceRule: value.recurrenceRule, VisibilityScope: value.visibilityScope,
		ScopeDepartmentID: optionalID(value.scopeDepartmentID), ScopeUserID: optionalID(value.scopeUserID),
		CompletedAt: optionalTimestamp(value.completedAt),
		CreatedBy:   mustID(value.createdBy), Version: value.version,
		CreatedAt: value.createdAt.Time.UTC(), UpdatedAt: value.updatedAt.Time.UTC(),
	}
}

func activityFromGet(row dbgen.GetActivityRow) Activity {
	return mapActivity(activityFields{row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType,
		row.RelatedID, row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt,
		row.EndsAt, row.Location, row.RecurrenceRule, row.VisibilityScope, row.ScopeDepartmentID,
		row.ScopeUserID, row.CompletedAt, row.CreatedBy, row.Version,
		row.CreatedAt, row.UpdatedAt})
}

func activityFromCreateAdvanced(row dbgen.CreateActivityRow) Activity {
	return mapActivity(activityFields{row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType,
		row.RelatedID, row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt,
		row.EndsAt, row.Location, row.RecurrenceRule, row.VisibilityScope, row.ScopeDepartmentID,
		row.ScopeUserID, row.CompletedAt, row.CreatedBy, row.Version,
		row.CreatedAt, row.UpdatedAt})
}

func activityFromUpdate(row dbgen.UpdateActivityRow) Activity {
	return mapActivity(activityFields{row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType,
		row.RelatedID, row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt,
		row.EndsAt, row.Location, row.RecurrenceRule, row.VisibilityScope, row.ScopeDepartmentID,
		row.ScopeUserID, row.CompletedAt, row.CreatedBy, row.Version,
		row.CreatedAt, row.UpdatedAt})
}

func activityFromTimeline(row dbgen.ListEntityTimelineRow) Activity {
	return mapActivity(activityFields{row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType,
		row.RelatedID, row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt,
		row.EndsAt, row.Location, row.RecurrenceRule, row.VisibilityScope, row.ScopeDepartmentID,
		row.ScopeUserID, row.CompletedAt, row.CreatedBy, row.Version,
		row.CreatedAt, row.UpdatedAt})
}

func activityFromFeed(row dbgen.ListActivityFeedRow) Activity {
	return mapActivity(activityFields{row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType,
		row.RelatedID, row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt,
		row.EndsAt, row.Location, row.RecurrenceRule, row.VisibilityScope, row.ScopeDepartmentID,
		row.ScopeUserID, row.CompletedAt, row.CreatedBy, row.Version,
		row.CreatedAt, row.UpdatedAt})
}

func activityFromCalendar(row dbgen.ListCalendarActivitiesRow) Activity {
	return mapActivity(activityFields{row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType,
		row.RelatedID, row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt,
		row.EndsAt, row.Location, row.RecurrenceRule, row.VisibilityScope, row.ScopeDepartmentID,
		row.ScopeUserID, row.CompletedAt, row.CreatedBy, row.Version,
		row.CreatedAt, row.UpdatedAt})
}

func commentFromCreate(row dbgen.CreateActivityCommentRow) Comment {
	return mapComment(row.ID, row.ActivityID, row.AuthorUserID, row.Body, row.MentionedUserIds,
		row.Version, row.CreatedAt, row.UpdatedAt)
}

func commentFromList(row dbgen.ListActivityCommentsRow) Comment {
	return mapComment(row.ID, row.ActivityID, row.AuthorUserID, row.Body, row.MentionedUserIds,
		row.Version, row.CreatedAt, row.UpdatedAt)
}

func commentFromUpdate(row dbgen.UpdateActivityCommentRow) Comment {
	return mapComment(row.ID, row.ActivityID, row.AuthorUserID, row.Body, row.MentionedUserIds,
		row.Version, row.CreatedAt, row.UpdatedAt)
}

func mapComment(
	id, activityID, authorID pgtype.UUID,
	body string,
	mentions []pgtype.UUID,
	version int64,
	createdAt, updatedAt pgtype.Timestamptz,
) Comment {
	mappedMentions := make([]ids.UUID, 0, len(mentions))
	for _, mention := range mentions {
		mappedMentions = append(mappedMentions, mustID(mention))
	}
	return Comment{
		ID: mustID(id), ActivityID: mustID(activityID), AuthorUserID: mustID(authorID),
		Body: body, MentionedUserIDs: mappedMentions, Version: version,
		CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC(),
	}
}

func reminderFromCreate(row dbgen.CreateActivityReminderRow) Reminder {
	return mapReminder(row.ID, row.ActivityID, row.RecipientUserID, row.RemindAt, row.Channel,
		row.DeliveredAt, row.CancelledAt, row.Version, row.CreatedAt)
}

func reminderFromList(row dbgen.ListActivityRemindersRow) Reminder {
	return mapReminder(row.ID, row.ActivityID, row.RecipientUserID, row.RemindAt, row.Channel,
		row.DeliveredAt, row.CancelledAt, row.Version, row.CreatedAt)
}

func reminderFromUpdate(row dbgen.UpdateActivityReminderRow) Reminder {
	return mapReminder(row.ID, row.ActivityID, row.RecipientUserID, row.RemindAt, row.Channel,
		row.DeliveredAt, row.CancelledAt, row.Version, row.CreatedAt)
}

func mapReminder(
	id, activityID, recipientID pgtype.UUID,
	remindAt pgtype.Timestamptz,
	channel string,
	deliveredAt, cancelledAt pgtype.Timestamptz,
	version int64,
	createdAt pgtype.Timestamptz,
) Reminder {
	return Reminder{
		ID: mustID(id), ActivityID: mustID(activityID), RecipientUserID: mustID(recipientID),
		RemindAt: remindAt.Time.UTC(), Channel: channel, DeliveredAt: optionalTimestamp(deliveredAt),
		CancelledAt: optionalTimestamp(cancelledAt), Version: version, CreatedAt: createdAt.Time.UTC(),
	}
}

func mustID(value pgtype.UUID) ids.UUID {
	result, _ := ids.FromPG(value)
	return result
}

func optionalID(value pgtype.UUID) *ids.UUID {
	result, valid := ids.FromPG(value)
	if !valid {
		return nil
	}
	return &result
}

func optionalTimestamp(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
