package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type jobPayload struct {
	ReminderID     string `json:"reminderId"`
	NotificationID string `json:"notificationId"`
	RecipientID    string `json:"recipientId"`
}

func WorkerHandlers(sender EmailSender, renderer TemplateRenderer) map[string]worker.Handler {
	return map[string]worker.Handler{
		"activity.reminder":  reminderHandler(),
		"notification.email": emailHandler(sender, renderer),
	}
}

func reminderHandler() worker.Handler {
	return func(ctx context.Context, dependencies worker.Dependencies, job worker.Job) error {
		payload, recipientID, err := decodeJobPayload(job, true)
		if err != nil {
			return err
		}
		reminderID, err := ids.Parse(payload.ReminderID)
		if err != nil {
			return codedJobError{"invalid_reminder_payload", "reminder ID is invalid"}
		}
		return withWorkerWorkspace(ctx, dependencies.AppPool, job, recipientID,
			func(workspace *tenancy.WorkspaceTx) error {
				row, err := workspace.Queries.LockActivityReminderForDelivery(ctx, dbgen.LockActivityReminderForDeliveryParams{
					WorkspaceID: job.WorkspaceID.PG(), ReminderID: reminderID.PG(),
				})
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				if err != nil {
					return fmt.Errorf("lock reminder: %w", err)
				}
				if row.RecipientUserID != recipientID.PG() {
					return codedJobError{"invalid_reminder_payload", "reminder recipient mismatch"}
				}
				if row.DeliveredAt.Valid || row.CancelledAt.Valid {
					return nil
				}
				delivery := DeliveryInApp
				if row.Channel == "email" {
					delivery = DeliveryEmail
				} else if row.Channel == "both" {
					delivery = DeliveryBoth
				}
				entityType := "activity"
				activityID := requiredID(row.ActivityID)
				if _, err := NewService().Create(ctx, workspace, job.WorkspaceID, Input{
					RecipientUserID: recipientID, MessageKey: "notifications.activity.reminder",
					MessageParams:   map[string]any{"title": row.ActivityTitle},
					TemplateVersion: 1, EntityType: &entityType, EntityID: &activityID, Delivery: delivery,
				}); err != nil {
					return err
				}
				updated, err := workspace.Queries.MarkActivityReminderDelivered(ctx, dbgen.MarkActivityReminderDeliveredParams{
					WorkspaceID: job.WorkspaceID.PG(), ReminderID: reminderID.PG(),
				})
				if err != nil {
					return fmt.Errorf("mark reminder delivered: %w", err)
				}
				if updated != 1 {
					return codedJobError{"reminder_state_conflict", "reminder state changed"}
				}
				return nil
			})
	}
}

func emailHandler(sender EmailSender, renderer TemplateRenderer) worker.Handler {
	return func(ctx context.Context, dependencies worker.Dependencies, job worker.Job) error {
		if sender == nil || renderer == nil {
			return codedJobError{"email_not_configured", "notification email transport is not configured"}
		}
		payload, recipientID, err := decodeJobPayload(job, false)
		if err != nil {
			return err
		}
		notificationID, err := ids.Parse(payload.NotificationID)
		if err != nil {
			return codedJobError{"invalid_notification_payload", "notification ID is invalid"}
		}
		return withWorkerWorkspace(ctx, dependencies.AppPool, job, recipientID,
			func(workspace *tenancy.WorkspaceTx) error {
				row, err := workspace.Queries.LockNotificationEmailDelivery(ctx, dbgen.LockNotificationEmailDeliveryParams{
					WorkspaceID: job.WorkspaceID.PG(), NotificationID: notificationID.PG(),
				})
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				if err != nil {
					return fmt.Errorf("lock notification email: %w", err)
				}
				if row.EmailState == "sent" || row.EmailState == "not_requested" {
					return nil
				}
				if row.EmailState != "queued" || requiredID(row.RecipientUserID) != recipientID {
					return codedJobError{"notification_state_conflict", "notification delivery state mismatch"}
				}
				params := map[string]any{}
				if err := json.Unmarshal(row.MessageParams, &params); err != nil {
					return codedJobError{"invalid_notification_params", "notification params are invalid"}
				}
				subject, body, err := renderer.Render(
					row.RecipientLocale, row.WorkspaceName, row.MessageKey, params,
				)
				if err != nil {
					return codedJobError{"notification_render_failed", err.Error()}
				}
				if err := sender.Send(ctx, EmailMessage{
					ID: notificationID.String(), Recipient: row.Email, Subject: subject, TextBody: body,
				}); err != nil {
					return codedJobError{"notification_email_failed", err.Error()}
				}
				updated, err := workspace.Queries.MarkNotificationEmailSent(ctx, dbgen.MarkNotificationEmailSentParams{
					WorkspaceID: job.WorkspaceID.PG(), NotificationID: notificationID.PG(),
				})
				if err != nil {
					return fmt.Errorf("mark notification email sent: %w", err)
				}
				if updated != 1 {
					return codedJobError{"notification_state_conflict", "notification delivery state changed"}
				}
				return nil
			})
	}
}

func withWorkerWorkspace(
	ctx context.Context,
	pool *pgxpool.Pool,
	job worker.Job,
	recipientID ids.UUID,
	operation func(*tenancy.WorkspaceTx) error,
) error {
	if pool == nil {
		return codedJobError{"worker_app_database_unavailable", "worker app database is unavailable"}
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin notification worker transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetActorContext(ctx, recipientID.String()); err != nil {
		return fmt.Errorf("set notification actor context: %w", err)
	}
	membership, err := queries.GetActiveMembership(ctx, dbgen.GetActiveMembershipParams{
		WorkspaceID: job.WorkspaceID.PG(), UserID: recipientID.PG(),
	})
	if err != nil {
		return fmt.Errorf("load notification membership: %w", err)
	}
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: job.WorkspaceID.String(), RequestID: "worker:" + job.ID.String(),
	}); err != nil {
		return fmt.Errorf("set notification tenant context: %w", err)
	}
	workspace := &tenancy.WorkspaceTx{Tx: tx, Queries: queries, Membership: membership}
	if err := operation(workspace); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit notification worker transaction: %w", err)
	}
	return nil
}

func decodeJobPayload(job worker.Job, reminder bool) (jobPayload, ids.UUID, error) {
	var payload jobPayload
	if job.SchemaVersion != 1 || json.Unmarshal(job.Payload, &payload) != nil {
		return jobPayload{}, ids.UUID{}, codedJobError{"invalid_notification_payload", "job payload is invalid"}
	}
	if (reminder && payload.ReminderID == "") || (!reminder && payload.NotificationID == "") {
		return jobPayload{}, ids.UUID{}, codedJobError{"invalid_notification_payload", "job target is missing"}
	}
	recipientID, err := ids.Parse(payload.RecipientID)
	if err != nil {
		return jobPayload{}, ids.UUID{}, codedJobError{"invalid_notification_payload", "recipient ID is invalid"}
	}
	return payload, recipientID, nil
}

type codedJobError struct {
	code    string
	message string
}

func (failure codedJobError) Error() string       { return failure.message }
func (failure codedJobError) FailureCode() string { return failure.code }
