package notifications

import (
	"context"
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Delivery string

const (
	DeliveryInApp Delivery = "in_app"
	DeliveryEmail Delivery = "email"
	DeliveryBoth  Delivery = "both"
)

type Input struct {
	RecipientUserID ids.UUID
	MessageKey      string
	MessageParams   map[string]any
	TemplateVersion int32
	EntityType      *string
	EntityID        *ids.UUID
	Delivery        Delivery
}

type Notification struct {
	ID              ids.UUID        `json:"id"`
	RecipientUserID ids.UUID        `json:"recipientUserId"`
	MessageKey      string          `json:"messageKey"`
	MessageParams   json.RawMessage `json:"messageParams"`
	TemplateVersion int32           `json:"templateVersion"`
	EntityType      *string         `json:"entityType,omitempty"`
	EntityID        *ids.UUID       `json:"entityId,omitempty"`
	ReadAt          *time.Time      `json:"readAt,omitempty"`
	Version         int64           `json:"version"`
	EmailState      string          `json:"emailState"`
	CreatedAt       time.Time       `json:"createdAt"`
}

type Page struct {
	Items      []Notification `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type EmailMessage struct {
	ID        string
	Recipient string
	Subject   string
	TextBody  string
}

type EmailSender interface {
	Send(ctx context.Context, message EmailMessage) error
}

type TemplateRenderer interface {
	Render(locale, workspaceName, messageKey string, params map[string]any) (subject, body string, err error)
}
