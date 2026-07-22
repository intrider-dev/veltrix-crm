package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/pagination"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) Create(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	input Input,
) (Notification, error) {
	input, params, err := validateInput(input)
	if err != nil {
		return Notification{}, err
	}
	membership, err := workspace.Queries.GetMembershipByUserID(ctx, dbgen.GetMembershipByUserIDParams{
		WorkspaceID: workspaceID.PG(), UserID: input.RecipientUserID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && membership.Status != "active") {
		return Notification{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/recipientUserId", Code: "validation.reference.invalid",
		}}}
	}
	if err != nil {
		return Notification{}, fmt.Errorf("validate notification recipient: %w", err)
	}
	notificationID, err := ids.NewV7()
	if err != nil {
		return Notification{}, err
	}
	emailState := "not_requested"
	if input.Delivery == DeliveryEmail || input.Delivery == DeliveryBoth {
		emailState = "queued"
	}
	row, err := workspace.Queries.CreateNotification(ctx, dbgen.CreateNotificationParams{
		WorkspaceID: workspaceID.PG(), NotificationID: notificationID.PG(),
		RecipientUserID: input.RecipientUserID.PG(), MessageKey: input.MessageKey,
		MessageParams: params, TemplateVersion: input.TemplateVersion,
		EntityType: input.EntityType, EntityID: optionalUUID(input.EntityID), EmailState: emailState,
	})
	if err != nil {
		return Notification{}, fmt.Errorf("create notification: %w", err)
	}
	if emailState == "queued" {
		jobID, jobErr := ids.NewV7()
		if jobErr != nil {
			return Notification{}, jobErr
		}
		if err := workspace.Queries.EnqueueNotificationEmailJob(ctx, dbgen.EnqueueNotificationEmailJobParams{
			WorkspaceID: workspaceID.PG(), JobID: jobID.PG(), NotificationID: notificationID.String(),
			RecipientUserID: input.RecipientUserID.String(),
		}); err != nil {
			return Notification{}, fmt.Errorf("enqueue notification email: %w", err)
		}
	}
	sseID, err := ids.NewV7()
	if err != nil {
		return Notification{}, err
	}
	data, err := json.Marshal(map[string]any{
		"id": notificationID.String(), "recipientUserId": input.RecipientUserID.String(),
		"messageKey": input.MessageKey, "messageParams": input.MessageParams,
		"entityType": input.EntityType, "entityId": uuidString(input.EntityID),
	})
	if err != nil {
		return Notification{}, fmt.Errorf("encode notification SSE event: %w", err)
	}
	if err := workspace.Queries.InsertUserSSEEvent(ctx, dbgen.InsertUserSSEEventParams{
		WorkspaceID: workspaceID.PG(), EventID: sseID.PG(), EventType: "notification.created", Data: data,
		RecipientUserID: input.RecipientUserID.PG(),
	}); err != nil {
		return Notification{}, fmt.Errorf("insert notification SSE event: %w", err)
	}
	return notificationFromCreate(row), nil
}

func (service *Service) List(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, recipientID ids.UUID,
	unreadOnly bool,
	cursor string,
	limit int,
) (Page, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	filter := fmt.Sprintf("notifications=%t:%s", unreadOnly, recipientID.String())
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return Page{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/query/cursor", Code: "validation.cursor.invalid",
		}}}
	}
	rows, err := workspace.Queries.ListUserNotifications(ctx, dbgen.ListUserNotificationsParams{
		WorkspaceID: workspaceID.PG(), RecipientUserID: recipientID.PG(), UnreadOnly: unreadOnly,
		CursorCreatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true},
		CursorID:        cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return Page{}, fmt.Errorf("list notifications: %w", err)
	}
	items := make([]Notification, 0, min(len(rows), limit))
	for index, row := range rows {
		if index == limit {
			break
		}
		items = append(items, notificationFromList(row))
	}
	page := Page{Items: items}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.CreatedAt.Time, lastID, filter)
		if err != nil {
			return Page{}, err
		}
	}
	return page, nil
}

func (service *Service) MarkRead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, recipientID, notificationID ids.UUID,
	expectedVersion int64,
) (Notification, error) {
	row, err := workspace.Queries.MarkNotificationRead(ctx, dbgen.MarkNotificationReadParams{
		WorkspaceID: workspaceID.PG(), NotificationID: notificationID.PG(),
		RecipientUserID: recipientID.PG(), ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, errx.ErrVersionConflict
	}
	if err != nil {
		return Notification{}, fmt.Errorf("mark notification read: %w", err)
	}
	return notificationFromRead(row), nil
}

func (service *Service) MarkAllRead(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, recipientID ids.UUID,
) (int64, error) {
	count, err := workspace.Queries.MarkAllNotificationsRead(ctx, dbgen.MarkAllNotificationsReadParams{
		WorkspaceID: workspaceID.PG(), RecipientUserID: recipientID.PG(),
	})
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return count, nil
}

func notificationFromCreate(row dbgen.CreateNotificationRow) Notification {
	return mapNotification(row.ID, row.RecipientUserID, row.MessageKey, row.MessageParams,
		row.TemplateVersion, row.EntityType, row.EntityID, row.ReadAt, row.Version,
		row.EmailState, row.CreatedAt)
}

func notificationFromList(row dbgen.ListUserNotificationsRow) Notification {
	return mapNotification(row.ID, row.RecipientUserID, row.MessageKey, row.MessageParams,
		row.TemplateVersion, row.EntityType, row.EntityID, row.ReadAt, row.Version,
		row.EmailState, row.CreatedAt)
}

func notificationFromRead(row dbgen.MarkNotificationReadRow) Notification {
	return mapNotification(row.ID, row.RecipientUserID, row.MessageKey, row.MessageParams,
		row.TemplateVersion, row.EntityType, row.EntityID, row.ReadAt, row.Version,
		row.EmailState, row.CreatedAt)
}

func mapNotification(
	id, recipientID pgtype.UUID,
	messageKey string,
	params []byte,
	templateVersion int32,
	entityType *string,
	entityID pgtype.UUID,
	readAt pgtype.Timestamptz,
	version int64,
	emailState string,
	createdAt pgtype.Timestamptz,
) Notification {
	return Notification{
		ID: requiredID(id), RecipientUserID: requiredID(recipientID), MessageKey: messageKey,
		MessageParams: append(json.RawMessage(nil), params...), TemplateVersion: templateVersion,
		EntityType: entityType, EntityID: nullableID(entityID), ReadAt: nullableTime(readAt),
		Version: version, EmailState: emailState, CreatedAt: createdAt.Time.UTC(),
	}
}

func requiredID(value pgtype.UUID) ids.UUID {
	result, _ := ids.FromPG(value)
	return result
}

func nullableID(value pgtype.UUID) *ids.UUID {
	result, ok := ids.FromPG(value)
	if !ok {
		return nil
	}
	return &result
}

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func optionalUUID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func uuidString(value *ids.UUID) any {
	if value == nil {
		return nil
	}
	return value.String()
}
