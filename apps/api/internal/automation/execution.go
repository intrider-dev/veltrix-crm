package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

const MaxExecutionDepth = 8

var ErrRecursionLimit = errors.New("automation recursion limit reached")

type ActionContext struct {
	Execution      Execution
	Index          int
	IdempotencyKey string
}

type ActionExecutor interface {
	Execute(context.Context, ActionContext, Action) (map[string]any, error)
}

// ExecutionRepository owns the transaction/RLS details. Begin must atomically
// transition queued/failed to running and return claimed=false for a completed
// or concurrently running execution, which makes duplicate job delivery safe.
type ExecutionRepository interface {
	Begin(context.Context, worker.Job, string) (Execution, bool, error)
	BeginAction(context.Context, Execution, int, ActionType, string) (map[string]any, bool, error)
	CompleteAction(context.Context, Execution, int, map[string]any) error
	FailAction(context.Context, Execution, int, string) error
	Complete(context.Context, Execution, []map[string]any) error
	Fail(context.Context, Execution, string, bool, []map[string]any) error
}

type executionFailure struct {
	code string
	err  error
}

func (failure executionFailure) Error() string       { return failure.err.Error() }
func (failure executionFailure) Unwrap() error       { return failure.err }
func (failure executionFailure) FailureCode() string { return failure.code }

func NewWorkerHandler(repository ExecutionRepository, executor ActionExecutor) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if repository == nil || executor == nil {
			return executionFailure{code: "automation_not_configured", err: errors.New("automation handler is not configured")}
		}
		var payload struct {
			ExecutionID string `json:"executionId"`
		}
		decoder := json.NewDecoder(bytes.NewReader(job.Payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil || payload.ExecutionID == "" {
			return executionFailure{code: "automation_payload_invalid", err: errors.New("invalid automation job payload")}
		}
		execution, claimed, err := repository.Begin(ctx, job, payload.ExecutionID)
		if err != nil {
			return executionFailure{code: "automation_begin_failed", err: err}
		}
		if !claimed {
			return nil
		}
		if execution.Depth >= MaxExecutionDepth {
			_ = repository.Fail(ctx, execution, "automation_recursion_limit", true, nil)
			return executionFailure{code: "automation_recursion_limit", err: ErrRecursionLimit}
		}
		results := make([]map[string]any, 0, len(execution.Actions))
		for index, action := range execution.Actions {
			idempotencyKey := execution.ID.String() + ":" + fmt.Sprint(index)
			previous, claimed, beginErr := repository.BeginAction(ctx, execution, index, action.Type, idempotencyKey)
			if beginErr != nil {
				_ = repository.Fail(ctx, execution, "automation_action_begin_failed", false, results)
				return executionFailure{code: "automation_action_begin_failed", err: beginErr}
			}
			if !claimed {
				results = append(results, previous)
				continue
			}
			result, actionErr := executor.Execute(ctx, ActionContext{
				Execution: execution, Index: index, IdempotencyKey: idempotencyKey,
			}, action)
			if actionErr != nil {
				_ = repository.FailAction(ctx, execution, index, "automation_action_failed")
				dead := execution.Attempts >= execution.MaxAttempts
				_ = repository.Fail(ctx, execution, "automation_action_failed", dead, results)
				return executionFailure{code: "automation_action_failed", err: fmt.Errorf("execute action %d: %w", index, actionErr)}
			}
			if result == nil {
				result = map[string]any{}
			}
			result["action"] = action.Type
			if err := repository.CompleteAction(ctx, execution, index, result); err != nil {
				_ = repository.FailAction(ctx, execution, index, "automation_action_complete_failed")
				_ = repository.Fail(ctx, execution, "automation_action_complete_failed", false, results)
				return executionFailure{code: "automation_action_complete_failed", err: err}
			}
			results = append(results, result)
		}
		if err := repository.Complete(ctx, execution, results); err != nil {
			_ = repository.Fail(ctx, execution, "automation_complete_failed", false, results)
			return executionFailure{code: "automation_complete_failed", err: err}
		}
		return nil
	}
}
