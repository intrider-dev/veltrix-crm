package automation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func TestValidateAndEvaluateNestedCondition(t *testing.T) {
	t.Parallel()
	condition := Condition{All: []Condition{
		{Field: "amount_minor", Operator: ComparatorGreaterOrEqual, Value: json.RawMessage(`10000`)},
		{Any: []Condition{
			{Field: "status", Operator: ComparatorEquals, Value: json.RawMessage(`"open"`)},
			{Field: "status", Operator: ComparatorEquals, Value: json.RawMessage(`"won"`)},
		}},
		{Field: "tags", Operator: ComparatorTagPresent, Value: json.RawMessage(`"018f1f6e-4d51-7a8c-8ae4-6f771c22c999"`)},
	}}
	spec := validSpec(condition)
	if err := ValidateRule(spec); err != nil {
		t.Fatalf("ValidateRule() error = %v", err)
	}
	matched, err := Evaluate(condition, Event{
		Fields: map[string]any{"amount_minor": int64(15000), "status": "open"},
		Tags:   []string{"018f1f6e-4d51-7a8c-8ae4-6f771c22c999"},
	})
	if err != nil || !matched {
		t.Fatalf("Evaluate() = %v, %v; want true, nil", matched, err)
	}
}

func TestValidateRuleRejectsAmbiguousAndProtectedMutation(t *testing.T) {
	t.Parallel()
	spec := validSpec(Condition{
		All:      []Condition{{Field: "status", Operator: ComparatorEquals, Value: json.RawMessage(`"open"`)}},
		Field:    "status",
		Operator: ComparatorEquals,
		Value:    json.RawMessage(`"open"`),
	})
	spec.Actions = []Action{{Type: ActionUpdateField, Params: json.RawMessage(`{"field":"workspace_id","value":"attacker"}`)}}
	if err := ValidateRule(spec); err == nil {
		t.Fatal("ValidateRule() accepted an ambiguous condition and protected-field mutation")
	}
}

func TestValidateRuleScopesDatabaseDrivenTriggers(t *testing.T) {
	t.Parallel()
	condition := Condition{Field: "scheduled_at", Operator: ComparatorDateAfter, Value: json.RawMessage(`"2025-01-01T00:00:00Z"`)}
	scheduled := validSpec(condition)
	scheduled.Trigger = TriggerScheduled
	scheduled.EntityType = EntityWorkspace
	if err := ValidateRule(scheduled); err != nil {
		t.Fatalf("scheduled workspace rule rejected: %v", err)
	}
	scheduled.EntityType = EntityContact
	if err := ValidateRule(scheduled); err == nil {
		t.Fatal("scheduled rule targeting a contact was accepted")
	}
	overdue := validSpec(Condition{Field: "status", Operator: ComparatorEquals, Value: json.RawMessage(`"open"`)})
	overdue.Trigger = TriggerTaskOverdue
	overdue.EntityType = EntityDeal
	if err := ValidateRule(overdue); err == nil {
		t.Fatal("task-overdue rule targeting a deal was accepted")
	}
}

func TestDispatchIsRateLimitedAndIdempotent(t *testing.T) {
	t.Parallel()
	ruleID := mustID(t)
	repository := &dispatchRepositoryStub{
		rules: []Rule{{ID: ruleID, Spec: validSpec(Condition{
			Field: "status", Operator: ComparatorEquals, Value: json.RawMessage(`"open"`),
		})}},
		outcomes: []QueueOutcome{QueueDuplicate},
	}
	event := Event{Trigger: TriggerRecordUpdated, Depth: 1, Fields: map[string]any{"status": "open"}}
	result, err := Dispatch(context.Background(), repository, event)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Matched != 1 || result.Duplicate != 1 || result.Queued != 0 {
		t.Fatalf("Dispatch() = %+v", result)
	}

	repository.rules = append(repository.rules, repository.rules[0])
	repository.outcomes = []QueueOutcome{QueueRateLimited, QueueRateLimited}
	result, err = Dispatch(context.Background(), repository, event)
	if err != nil {
		t.Fatalf("second Dispatch() error = %v", err)
	}
	if result.RateLimited != 2 {
		t.Fatalf("second Dispatch().RateLimited = %d, want 2", result.RateLimited)
	}
}

func TestExecutionHandlerStopsAtRecursionLimit(t *testing.T) {
	t.Parallel()
	repository := &executionRepositoryStub{execution: Execution{Depth: MaxExecutionDepth, MaxAttempts: 8}}
	handler := NewWorkerHandler(repository, actionExecutorStub{})
	err := handler(context.Background(), worker.Dependencies{}, worker.Job{Payload: json.RawMessage(`{"executionId":"execution"}`)})
	if !errors.Is(err, ErrRecursionLimit) {
		t.Fatalf("handler error = %v, want ErrRecursionLimit", err)
	}
	if repository.failureCode != "automation_recursion_limit" || !repository.dead {
		t.Fatalf("failure = %q dead=%v", repository.failureCode, repository.dead)
	}
}

func TestExecutionHandlerCompletesActionsInOrder(t *testing.T) {
	t.Parallel()
	repository := &executionRepositoryStub{execution: Execution{
		Actions: []Action{{Type: ActionAddTag, Params: json.RawMessage(`{"tagId":"018f1f6e-4d51-7a8c-8ae4-6f771c22c999"}`)},
			{Type: ActionCreateNotification, Params: json.RawMessage(`{"recipientId":"018f1f6e-4d51-7a8c-8ae4-6f771c22c999","messageKey":"automation.test"}`)}},
		MaxAttempts: 8,
	}}
	executor := &recordingExecutor{}
	handler := NewWorkerHandler(repository, executor)
	if err := handler(context.Background(), worker.Dependencies{}, worker.Job{Payload: json.RawMessage(`{"executionId":"execution"}`)}); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if len(executor.types) != 2 || executor.types[0] != ActionAddTag || executor.types[1] != ActionCreateNotification {
		t.Fatalf("execution order = %v", executor.types)
	}
	if !repository.completed {
		t.Fatal("execution was not completed")
	}
}

func validSpec(condition Condition) RuleSpec {
	return RuleSpec{
		Name: "High-value deal", Trigger: TriggerRecordUpdated, EntityType: EntityDeal,
		Conditions: condition, RateLimitPerHour: 100,
		Actions: []Action{{Type: ActionCreateNotification, Params: json.RawMessage(
			`{"recipientId":"018f1f6e-4d51-7a8c-8ae4-6f771c22c999","messageKey":"automation.test"}`,
		)}},
	}
}

func mustID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type dispatchRepositoryStub struct {
	rules    []Rule
	outcomes []QueueOutcome
}

func (stub *dispatchRepositoryStub) ListEnabled(context.Context, Event) ([]Rule, error) {
	return stub.rules, nil
}
func (stub *dispatchRepositoryStub) ReserveAndEnqueue(context.Context, Rule, Event) (QueueOutcome, error) {
	if len(stub.outcomes) == 0 {
		return QueueDuplicate, nil
	}
	outcome := stub.outcomes[0]
	stub.outcomes = stub.outcomes[1:]
	return outcome, nil
}

type executionRepositoryStub struct {
	execution   Execution
	completed   bool
	failureCode string
	dead        bool
}

func (stub *executionRepositoryStub) Begin(context.Context, worker.Job, string) (Execution, bool, error) {
	return stub.execution, true, nil
}
func (stub *executionRepositoryStub) Complete(context.Context, Execution, []map[string]any) error {
	stub.completed = true
	return nil
}
func (stub *executionRepositoryStub) BeginAction(context.Context, Execution, int, ActionType, string) (map[string]any, bool, error) {
	return nil, true, nil
}
func (stub *executionRepositoryStub) CompleteAction(context.Context, Execution, int, map[string]any) error {
	return nil
}
func (stub *executionRepositoryStub) FailAction(context.Context, Execution, int, string) error {
	return nil
}
func (stub *executionRepositoryStub) Fail(_ context.Context, _ Execution, code string, dead bool, _ []map[string]any) error {
	stub.failureCode, stub.dead = code, dead
	return nil
}

type actionExecutorStub struct{}

func (actionExecutorStub) Execute(context.Context, ActionContext, Action) (map[string]any, error) {
	return nil, nil
}

type recordingExecutor struct{ types []ActionType }

func (executor *recordingExecutor) Execute(_ context.Context, _ ActionContext, action Action) (map[string]any, error) {
	executor.types = append(executor.types, action.Type)
	return map[string]any{"ok": true}, nil
}
