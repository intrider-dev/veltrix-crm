package automation

import (
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type TriggerType string

const (
	TriggerRecordCreated    TriggerType = "record_created"
	TriggerRecordUpdated    TriggerType = "record_updated"
	TriggerDealStageChanged TriggerType = "deal_stage_changed"
	TriggerDealWon          TriggerType = "deal_won"
	TriggerDealLost         TriggerType = "deal_lost"
	TriggerTaskOverdue      TriggerType = "task_overdue"
	TriggerScheduled        TriggerType = "scheduled"
)

type EntityType string

const (
	EntityContact   EntityType = "contact"
	EntityCompany   EntityType = "company"
	EntityLead      EntityType = "lead"
	EntityDeal      EntityType = "deal"
	EntityActivity  EntityType = "activity"
	EntityWorkspace EntityType = "workspace"
)

type Comparator string

const (
	ComparatorEquals         Comparator = "equals"
	ComparatorNotEquals      Comparator = "not_equals"
	ComparatorContains       Comparator = "contains"
	ComparatorGreaterThan    Comparator = "greater_than"
	ComparatorGreaterOrEqual Comparator = "greater_or_equal"
	ComparatorLessThan       Comparator = "less_than"
	ComparatorLessOrEqual    Comparator = "less_or_equal"
	ComparatorDateBefore     Comparator = "date_before"
	ComparatorDateAfter      Comparator = "date_after"
	ComparatorTagPresent     Comparator = "tag_present"
	ComparatorOwnerEquals    Comparator = "owner_equals"
	ComparatorTeamEquals     Comparator = "team_equals"
)

// Condition is a bounded expression tree. Exactly one of All, Any, or the
// predicate fields must be present. Empty all/any groups are rejected so a
// malformed rule can never match everything accidentally.
type Condition struct {
	All      []Condition     `json:"all,omitempty"`
	Any      []Condition     `json:"any,omitempty"`
	Field    string          `json:"field,omitempty"`
	Operator Comparator      `json:"operator,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type ActionType string

const (
	ActionCreateTask         ActionType = "create_task"
	ActionAssignOwner        ActionType = "assign_owner"
	ActionAddTag             ActionType = "add_tag"
	ActionRemoveTag          ActionType = "remove_tag"
	ActionCreateNotification ActionType = "create_notification"
	ActionSendEmail          ActionType = "send_email"
	ActionInvokeWebhook      ActionType = "invoke_webhook"
	ActionUpdateField        ActionType = "update_field"
)

type Action struct {
	Type   ActionType      `json:"type"`
	Params json.RawMessage `json:"params"`
}

type CreateTaskParams struct {
	TitleKey    string         `json:"titleKey"`
	TitleParams map[string]any `json:"titleParams,omitempty"`
	AssigneeID  string         `json:"assigneeId,omitempty"`
	DueInHours  int            `json:"dueInHours,omitempty"`
	Priority    string         `json:"priority,omitempty"`
}

type AssignOwnerParams struct {
	OwnerID string `json:"ownerId"`
}

type TagParams struct {
	TagID string `json:"tagId"`
}

type CreateNotificationParams struct {
	RecipientID   string         `json:"recipientId"`
	MessageKey    string         `json:"messageKey"`
	MessageParams map[string]any `json:"messageParams,omitempty"`
}

type SendEmailParams struct {
	RecipientField string         `json:"recipientField"`
	TemplateKey    string         `json:"templateKey"`
	TemplateParams map[string]any `json:"templateParams,omitempty"`
}

type InvokeWebhookParams struct {
	SubscriptionID string `json:"subscriptionId"`
}

type UpdateFieldParams struct {
	Field string `json:"field"`
	Value any    `json:"value"`
}

type RuleSpec struct {
	Name             string      `json:"name"`
	Trigger          TriggerType `json:"trigger"`
	EntityType       EntityType  `json:"entityType"`
	Conditions       Condition   `json:"conditions"`
	Actions          []Action    `json:"actions"`
	Enabled          bool        `json:"enabled"`
	RateLimitPerHour int         `json:"rateLimitPerHour"`
}

type Rule struct {
	WorkspaceID ids.UUID
	ID          ids.UUID
	CreatedBy   ids.UUID
	Spec        RuleSpec
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Event struct {
	WorkspaceID   ids.UUID
	EventID       ids.UUID
	CorrelationID ids.UUID
	ActorID       ids.UUID
	Trigger       TriggerType
	EntityType    EntityType
	EntityID      ids.UUID
	Depth         int
	Fields        map[string]any
	Tags          []string
	OwnerID       string
	TeamID        string
}

type Preview struct {
	Matched bool         `json:"matched"`
	Actions []ActionType `json:"actions"`
}

type Execution struct {
	WorkspaceID   ids.UUID
	ID            ids.UUID
	RuleID        ids.UUID
	EventID       ids.UUID
	CorrelationID ids.UUID
	ActorID       ids.UUID
	Depth         int
	Trigger       TriggerType
	Event         Event
	Actions       []Action
	Attempts      int
	MaxAttempts   int
	State         string
}
