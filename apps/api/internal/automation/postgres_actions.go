package automation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/idempotency"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type PostgresActionPorts struct {
	pool *pgxpool.Pool
}

func NewPostgresActionPorts(pool *pgxpool.Pool) *PostgresActionPorts {
	return &PostgresActionPorts{pool: pool}
}

type actionOperation func(context.Context, pgx.Tx, *dbgen.Queries, events.Metadata) (map[string]any, error)

func (ports *PostgresActionPorts) run(
	ctx context.Context, target Target, key, operation string, request any, fn actionOperation,
) (map[string]any, error) {
	if ports == nil || ports.pool == nil {
		return nil, errors.New("automation action database pool is required")
	}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	tx, err := ports.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetActorContext(ctx, target.ActorID.String()); err != nil {
		return nil, err
	}
	if _, err := queries.GetActiveMembership(ctx, dbgen.GetActiveMembershipParams{
		WorkspaceID: target.WorkspaceID.PG(), UserID: target.ActorID.PG(),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errx.ErrForbidden
		}
		return nil, err
	}
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: target.WorkspaceID.String(), RequestID: key,
	}); err != nil {
		return nil, err
	}
	replay, err := idempotency.Reserve(ctx, queries, target.WorkspaceID, target.ActorID, key, operation, requestBody)
	if err != nil {
		return nil, err
	}
	if replay != nil {
		result := map[string]any{}
		if err := json.Unmarshal(replay.Body, &result); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}
	metadata := events.Metadata{
		WorkspaceID: target.WorkspaceID, ActorID: target.ActorID,
		RequestID: key, CorrelationID: target.CorrelationID,
	}
	result, err := fn(ctx, tx, queries, metadata)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err := idempotency.Complete(ctx, queries, target.WorkspaceID, key, 200, encoded); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (ports *PostgresActionPorts) CreateTask(
	ctx context.Context, target Target, key string, params CreateTaskParams,
) (ids.UUID, error) {
	result, err := ports.run(ctx, target, key, "automation.create_task", params,
		func(ctx context.Context, tx pgx.Tx, queries *dbgen.Queries, metadata events.Metadata) (map[string]any, error) {
			taskID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			assigneeID := target.ActorID
			if params.AssigneeID != "" {
				assigneeID, err = ids.Parse(params.AssigneeID)
				if err != nil {
					return nil, err
				}
			}
			priority := params.Priority
			if priority == "" {
				priority = "normal"
			}
			var relatedType *string
			var relatedID pgtype.UUID
			if target.EntityType == EntityContact || target.EntityType == EntityCompany || target.EntityType == EntityDeal {
				value := string(target.EntityType)
				relatedType, relatedID = &value, target.EntityID.PG()
			}
			titleParams, err := json.Marshal(params.TitleParams)
			if err != nil {
				return nil, err
			}
			var dueAt pgtype.Timestamptz
			if params.DueInHours > 0 {
				dueAt = pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Duration(params.DueInHours) * time.Hour), Valid: true}
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO activities.activities (
				  workspace_id, id, activity_type, title, system_title_key,
				  system_title_params, related_type, related_id, assignee_user_id,
				  priority, due_at, created_by
				) VALUES ($1, $2, 'task', $3, $3, $4, $5, $6, $7, $8, $9, $10)`,
				target.WorkspaceID.PG(), taskID.PG(), params.TitleKey, titleParams,
				relatedType, relatedID, assigneeID.PG(), priority, dueAt, target.ActorID.PG())
			if err != nil {
				return nil, err
			}
			if err := events.Record(ctx, queries, metadata, events.Mutation{
				Action: "automation.task.created", EventType: "activities.activity.created",
				AggregateType: "activity", AggregateID: taskID,
				Summary: map[string]any{"systemTitleKey": params.TitleKey},
				Payload: map[string]any{"activityId": taskID.String(), "type": "task", "automationDepth": target.Depth},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"id": taskID.String()}, nil
		})
	return resultID(result, err)
}

func (ports *PostgresActionPorts) AssignOwner(
	ctx context.Context, target Target, key string, params AssignOwnerParams,
) error {
	ownerID, err := ids.Parse(params.OwnerID)
	if err != nil {
		return err
	}
	_, err = ports.run(ctx, target, key, "automation.assign_owner", params,
		func(ctx context.Context, tx pgx.Tx, queries *dbgen.Queries, metadata events.Metadata) (map[string]any, error) {
			query, ok := ownerUpdateQuery(target.EntityType)
			if !ok {
				return nil, actionValidation("/params", "automation.target.unsupported")
			}
			command, err := tx.Exec(ctx, query, target.WorkspaceID.PG(), target.EntityID.PG(), ownerID.PG())
			if err != nil {
				return nil, err
			}
			if command.RowsAffected() != 1 {
				return nil, errx.ErrNotFound
			}
			if err := recordTargetMutation(ctx, queries, metadata, target, "owner.assigned", map[string]any{"ownerId": ownerID.String()}); err != nil {
				return nil, err
			}
			return map[string]any{"updated": true}, nil
		})
	return err
}

func (ports *PostgresActionPorts) AddTag(ctx context.Context, target Target, key string, params TagParams) error {
	return ports.changeTag(ctx, target, key, params, true)
}

func (ports *PostgresActionPorts) RemoveTag(ctx context.Context, target Target, key string, params TagParams) error {
	return ports.changeTag(ctx, target, key, params, false)
}

func (ports *PostgresActionPorts) changeTag(
	ctx context.Context, target Target, key string, params TagParams, add bool,
) error {
	if target.EntityType != EntityContact {
		return actionValidation("/params", "automation.target.unsupported")
	}
	tagID, err := ids.Parse(params.TagID)
	if err != nil {
		return err
	}
	operation := "automation.remove_tag"
	if add {
		operation = "automation.add_tag"
	}
	_, err = ports.run(ctx, target, key, operation, params,
		func(ctx context.Context, tx pgx.Tx, queries *dbgen.Queries, metadata events.Metadata) (map[string]any, error) {
			var err error
			if add {
				_, err = tx.Exec(ctx, `INSERT INTO customers.contact_tags (workspace_id, contact_id, tag_id)
					VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, target.WorkspaceID.PG(), target.EntityID.PG(), tagID.PG())
			} else {
				_, err = tx.Exec(ctx, `DELETE FROM customers.contact_tags
					WHERE workspace_id=$1 AND contact_id=$2 AND tag_id=$3`, target.WorkspaceID.PG(), target.EntityID.PG(), tagID.PG())
			}
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `UPDATE customers.contacts SET version=version+1, updated_at=now()
				WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`, target.WorkspaceID.PG(), target.EntityID.PG()); err != nil {
				return nil, err
			}
			verb := "tag.removed"
			if add {
				verb = "tag.added"
			}
			if err := recordTargetMutation(ctx, queries, metadata, target, verb, map[string]any{"tagId": tagID.String()}); err != nil {
				return nil, err
			}
			return map[string]any{"updated": true}, nil
		})
	return err
}

func (ports *PostgresActionPorts) CreateNotification(
	ctx context.Context, target Target, key string, params CreateNotificationParams,
) (ids.UUID, error) {
	result, err := ports.run(ctx, target, key, "automation.create_notification", params,
		func(ctx context.Context, tx pgx.Tx, queries *dbgen.Queries, metadata events.Metadata) (map[string]any, error) {
			recipientID, err := ids.Parse(params.RecipientID)
			if err != nil {
				return nil, err
			}
			notificationID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			messageParams, err := json.Marshal(params.MessageParams)
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO notifications.notifications (
				workspace_id,id,recipient_user_id,message_key,message_params,entity_type,entity_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7)`, target.WorkspaceID.PG(), notificationID.PG(),
				recipientID.PG(), params.MessageKey, messageParams, string(target.EntityType), target.EntityID.PG())
			if err != nil {
				return nil, err
			}
			sseID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			ssePayload, _ := json.Marshal(map[string]any{
				"notificationId": notificationID.String(), "recipientUserId": recipientID.String(),
				"messageKey": params.MessageKey, "messageParams": params.MessageParams,
			})
			if err := queries.InsertUserSSEEvent(ctx, dbgen.InsertUserSSEEventParams{
				WorkspaceID: target.WorkspaceID.PG(), EventID: sseID.PG(), EventType: "notification.created", Data: ssePayload,
				RecipientUserID: recipientID.PG(),
			}); err != nil {
				return nil, err
			}
			if err := events.Record(ctx, queries, metadata, events.Mutation{
				Action: "automation.notification.created", EventType: "notifications.notification.created",
				AggregateType: "notification", AggregateID: notificationID,
				Summary: map[string]any{"messageKey": params.MessageKey},
				Payload: map[string]any{"notificationId": notificationID.String()},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"id": notificationID.String()}, nil
		})
	return resultID(result, err)
}

func (ports *PostgresActionPorts) SendEmail(
	ctx context.Context, target Target, key string, params SendEmailParams,
) (ids.UUID, error) {
	result, err := ports.run(ctx, target, key, "automation.send_email", params,
		func(ctx context.Context, _ pgx.Tx, queries *dbgen.Queries, _ events.Metadata) (map[string]any, error) {
			jobID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			payload, err := json.Marshal(map[string]any{
				"targetType": target.EntityType, "targetId": target.EntityID.String(),
				"recipientField": params.RecipientField, "templateKey": params.TemplateKey,
				"templateParams": params.TemplateParams,
			})
			if err != nil {
				return nil, err
			}
			if err := queries.InsertFanoutJob(ctx, dbgen.InsertFanoutJobParams{
				WorkspaceID: target.WorkspaceID.PG(), ID: jobID.PG(), Kind: "automation.email.send",
				SchemaVersion: 1, IdempotencyKey: key, Payload: payload, MaxAttempts: 8,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"id": jobID.String()}, nil
		})
	return resultID(result, err)
}

func (ports *PostgresActionPorts) InvokeWebhook(
	ctx context.Context, target Target, key string, params InvokeWebhookParams,
) (ids.UUID, error) {
	result, err := ports.run(ctx, target, key, "automation.invoke_webhook", params,
		func(ctx context.Context, _ pgx.Tx, queries *dbgen.Queries, metadata events.Metadata) (map[string]any, error) {
			subscriptionID, err := ids.Parse(params.SubscriptionID)
			if err != nil {
				return nil, err
			}
			subscription, err := queries.GetWebhookSubscriptionForDelivery(ctx, dbgen.GetWebhookSubscriptionForDeliveryParams{
				WorkspaceID: target.WorkspaceID.PG(), ID: subscriptionID.PG(),
			})
			if err != nil || !subscription.Enabled {
				if errors.Is(err, pgx.ErrNoRows) || (err == nil && !subscription.Enabled) {
					return nil, errx.ErrNotFound
				}
				return nil, err
			}
			eventID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			payload, _ := json.Marshal(map[string]any{
				"entityType": target.EntityType, "entityId": target.EntityID.String(), "automationDepth": target.Depth,
			})
			if err := queries.InsertOutboxEvent(ctx, dbgen.InsertOutboxEventParams{
				WorkspaceID: target.WorkspaceID.PG(), ID: eventID.PG(), EventType: "automation.webhook.invoked",
				SchemaVersion: 1, AggregateType: string(target.EntityType), AggregateID: target.EntityID.PG(),
				CausationID: pgtype.UUID{}, CorrelationID: target.CorrelationID.PG(), Payload: payload,
			}); err != nil {
				return nil, err
			}
			if _, err := queries.MarkOutboxPublished(ctx, dbgen.MarkOutboxPublishedParams{
				WorkspaceID: target.WorkspaceID.PG(), ID: eventID.PG(),
			}); err != nil {
				return nil, err
			}
			deliveryID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			if _, err := queries.CreateWebhookDelivery(ctx, dbgen.CreateWebhookDeliveryParams{
				WorkspaceID: target.WorkspaceID.PG(), ID: deliveryID.PG(),
				SubscriptionID: subscriptionID.PG(), EventID: eventID.PG(),
			}); err != nil {
				return nil, err
			}
			jobID, err := ids.NewV7()
			if err != nil {
				return nil, err
			}
			jobPayload, _ := json.Marshal(map[string]string{"deliveryId": deliveryID.String()})
			if err := queries.EnqueueWebhookDelivery(ctx, dbgen.EnqueueWebhookDeliveryParams{
				WorkspaceID: target.WorkspaceID.PG(), ID: jobID.PG(), IdempotencyKey: deliveryID.String(),
				Payload: jobPayload, MaxAttempts: subscription.MaxAttempts,
			}); err != nil {
				return nil, err
			}
			if err := events.Record(ctx, queries, metadata, events.Mutation{
				Action: "automation.webhook.queued", EventType: "automation.webhook.queued",
				AggregateType: "webhook_delivery", AggregateID: deliveryID,
				Summary: map[string]any{"subscriptionId": subscriptionID.String()},
				Payload: map[string]any{"deliveryId": deliveryID.String()},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"id": deliveryID.String()}, nil
		})
	return resultID(result, err)
}

func (ports *PostgresActionPorts) UpdateField(
	ctx context.Context, target Target, key string, params UpdateFieldParams,
) error {
	value, ok := params.Value.(string)
	if !ok || len(value) > 500 {
		return actionValidation("/params/value", "validation.string")
	}
	_, err := ports.run(ctx, target, key, "automation.update_field", params,
		func(ctx context.Context, tx pgx.Tx, queries *dbgen.Queries, metadata events.Metadata) (map[string]any, error) {
			query, ok := allowedFieldUpdateQuery(target.EntityType, params.Field)
			if !ok {
				return nil, actionValidation("/params/field", "automation.field.protected")
			}
			command, err := tx.Exec(ctx, query, target.WorkspaceID.PG(), target.EntityID.PG(), value)
			if err != nil {
				return nil, err
			}
			if command.RowsAffected() != 1 {
				return nil, errx.ErrNotFound
			}
			if err := recordTargetMutation(ctx, queries, metadata, target, "field.updated", map[string]any{"field": params.Field}); err != nil {
				return nil, err
			}
			return map[string]any{"updated": true}, nil
		})
	return err
}

func ownerUpdateQuery(entityType EntityType) (string, bool) {
	switch entityType {
	case EntityContact:
		return `UPDATE customers.contacts SET owner_user_id=$3,version=version+1,updated_at=now()
			WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`, true
	case EntityCompany:
		return `UPDATE customers.companies SET owner_user_id=$3,version=version+1,updated_at=now()
			WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`, true
	case EntityLead:
		return `UPDATE sales.leads SET owner_user_id=$3,version=version+1,updated_at=now()
			WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`, true
	case EntityDeal:
		return `UPDATE sales.deals SET owner_user_id=$3,version=version+1,updated_at=now()
			WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`, true
	default:
		return "", false
	}
}

func allowedFieldUpdateQuery(entityType EntityType, field string) (string, bool) {
	queries := map[EntityType]map[string]string{
		EntityContact: {
			"status":    `UPDATE customers.contacts SET status=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
			"source":    `UPDATE customers.contacts SET source=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
			"job_title": `UPDATE customers.contacts SET job_title=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
		},
		EntityCompany: {
			"status":   `UPDATE customers.companies SET status=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
			"industry": `UPDATE customers.companies SET industry=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
		},
		EntityLead: {
			"source": `UPDATE sales.leads SET source=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
		},
		EntityDeal: {
			"lost_reason": `UPDATE sales.deals SET lost_reason=$3,version=version+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
		},
	}
	query, ok := queries[entityType][field]
	return query, ok
}

func recordTargetMutation(
	ctx context.Context, queries *dbgen.Queries, metadata events.Metadata,
	target Target, action string, summary map[string]any,
) error {
	payload := map[string]any{
		"entityId": target.EntityID.String(), "automationDepth": target.Depth,
	}
	return events.Record(ctx, queries, metadata, events.Mutation{
		Action: "automation." + action, EventType: "automation." + string(target.EntityType) + ".updated",
		AggregateType: string(target.EntityType), AggregateID: target.EntityID,
		Summary: summary, Payload: payload,
	})
}

func resultID(result map[string]any, err error) (ids.UUID, error) {
	if err != nil {
		return ids.UUID{}, err
	}
	value, _ := result["id"].(string)
	if strings.TrimSpace(value) == "" {
		return ids.UUID{}, errors.New("automation action result has no ID")
	}
	return ids.Parse(value)
}

func actionValidation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}

var _ ActionPorts = (*PostgresActionPorts)(nil)
