package activities

import (
	"context"
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type AdvancedInput struct {
	Type              string
	Title             string
	Body              *string
	RelatedType       *string
	RelatedID         *ids.UUID
	AssigneeID        *ids.UUID
	Status            string
	Priority          string
	DueAt             *time.Time
	OccurredAt        time.Time
	EndsAt            *time.Time
	Location          *string
	RecurrenceRule    *string
	VisibilityScope   string
	ScopeDepartmentID *ids.UUID
	ScopeUserID       *ids.UUID
}

// Input is retained for the original vertical-slice handler. Both the compact
// and full activity endpoints now execute the same validation and write path.
type Input = AdvancedInput

type Activity struct {
	ID                ids.UUID   `json:"id"`
	Type              string     `json:"type"`
	Title             string     `json:"title"`
	Body              *string    `json:"body,omitempty"`
	RelatedType       *string    `json:"relatedType,omitempty"`
	RelatedID         *ids.UUID  `json:"relatedId,omitempty"`
	AssigneeID        *ids.UUID  `json:"assigneeId,omitempty"`
	Status            string     `json:"status"`
	Priority          string     `json:"priority"`
	DueAt             *time.Time `json:"dueAt,omitempty"`
	OccurredAt        time.Time  `json:"occurredAt"`
	EndsAt            *time.Time `json:"endsAt,omitempty"`
	Location          *string    `json:"location,omitempty"`
	RecurrenceRule    *string    `json:"recurrenceRule,omitempty"`
	VisibilityScope   string     `json:"visibilityScope"`
	ScopeDepartmentID *ids.UUID  `json:"scopeDepartmentId,omitempty"`
	ScopeUserID       *ids.UUID  `json:"scopeUserId,omitempty"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	CreatedBy         ids.UUID   `json:"createdBy"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ActivityPage struct {
	Items      []Activity `json:"items"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type CommentInput struct {
	Body             string
	MentionedUserIDs []ids.UUID
}

type Comment struct {
	ID               ids.UUID   `json:"id"`
	ActivityID       ids.UUID   `json:"activityId"`
	AuthorUserID     ids.UUID   `json:"authorUserId"`
	Body             string     `json:"body"`
	MentionedUserIDs []ids.UUID `json:"mentionedUserIds"`
	Version          int64      `json:"version"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type CommentPage struct {
	Items      []Comment `json:"items"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

type ReminderInput struct {
	RecipientUserID ids.UUID
	RemindAt        time.Time
	Channel         string
}

type Reminder struct {
	ID              ids.UUID   `json:"id"`
	ActivityID      ids.UUID   `json:"activityId"`
	RecipientUserID ids.UUID   `json:"recipientUserId"`
	RemindAt        time.Time  `json:"remindAt"`
	Channel         string     `json:"channel"`
	DeliveredAt     *time.Time `json:"deliveredAt,omitempty"`
	CancelledAt     *time.Time `json:"cancelledAt,omitempty"`
	Version         int64      `json:"version"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type MentionNotification struct {
	RecipientUserID ids.UUID
	MessageKey      string
	MessageParams   json.RawMessage
	ActivityID      ids.UUID
}

type MentionNotifier func(context.Context, MentionNotification) error
