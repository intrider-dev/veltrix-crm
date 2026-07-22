package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Target struct {
	WorkspaceID   ids.UUID
	EntityType    EntityType
	EntityID      ids.UUID
	ActorID       ids.UUID
	CorrelationID ids.UUID
	Depth         int
}

type ActionPorts interface {
	CreateTask(context.Context, Target, string, CreateTaskParams) (ids.UUID, error)
	AssignOwner(context.Context, Target, string, AssignOwnerParams) error
	AddTag(context.Context, Target, string, TagParams) error
	RemoveTag(context.Context, Target, string, TagParams) error
	CreateNotification(context.Context, Target, string, CreateNotificationParams) (ids.UUID, error)
	SendEmail(context.Context, Target, string, SendEmailParams) (ids.UUID, error)
	InvokeWebhook(context.Context, Target, string, InvokeWebhookParams) (ids.UUID, error)
	UpdateField(context.Context, Target, string, UpdateFieldParams) error
}

// TypedActionExecutor is the single dispatch point from validated action JSON
// to domain ports. Every port receives a stable idempotency key; external
// adapters must propagate it to their queue/provider boundary.
type TypedActionExecutor struct {
	ports ActionPorts
}

func NewTypedActionExecutor(ports ActionPorts) *TypedActionExecutor {
	return &TypedActionExecutor{ports: ports}
}

func (executor *TypedActionExecutor) Execute(
	ctx context.Context, actionContext ActionContext, action Action,
) (map[string]any, error) {
	if executor == nil || executor.ports == nil {
		return nil, errors.New("automation action ports are not configured")
	}
	if err := ValidateAction(action); err != nil {
		return nil, err
	}
	target := Target{
		WorkspaceID:   actionContext.Execution.WorkspaceID,
		EntityType:    actionContext.Execution.Event.EntityType,
		EntityID:      actionContext.Execution.Event.EntityID,
		ActorID:       actionContext.Execution.ActorID,
		CorrelationID: actionContext.Execution.CorrelationID,
		Depth:         actionContext.Execution.Depth,
	}
	key := actionContext.IdempotencyKey
	switch action.Type {
	case ActionCreateTask:
		var params CreateTaskParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		id, err := executor.ports.CreateTask(ctx, target, key, params)
		return actionIDResult("taskId", id, err)
	case ActionAssignOwner:
		var params AssignOwnerParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		return emptyResult(executor.ports.AssignOwner(ctx, target, key, params))
	case ActionAddTag:
		var params TagParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		return emptyResult(executor.ports.AddTag(ctx, target, key, params))
	case ActionRemoveTag:
		var params TagParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		return emptyResult(executor.ports.RemoveTag(ctx, target, key, params))
	case ActionCreateNotification:
		var params CreateNotificationParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		id, err := executor.ports.CreateNotification(ctx, target, key, params)
		return actionIDResult("notificationId", id, err)
	case ActionSendEmail:
		var params SendEmailParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		id, err := executor.ports.SendEmail(ctx, target, key, params)
		return actionIDResult("emailJobId", id, err)
	case ActionInvokeWebhook:
		var params InvokeWebhookParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		id, err := executor.ports.InvokeWebhook(ctx, target, key, params)
		return actionIDResult("webhookDeliveryId", id, err)
	case ActionUpdateField:
		var params UpdateFieldParams
		if err := json.Unmarshal(action.Params, &params); err != nil {
			return nil, err
		}
		return emptyResult(executor.ports.UpdateField(ctx, target, key, params))
	default:
		return nil, fmt.Errorf("unsupported automation action %q", action.Type)
	}
}

func actionIDResult(key string, id ids.UUID, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]any{key: id.String()}, nil
}

func emptyResult(err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]any{"updated": true}, nil
}
