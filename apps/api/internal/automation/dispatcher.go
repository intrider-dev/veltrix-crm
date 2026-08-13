package automation

import (
	"context"
	"errors"
	"fmt"
)

type DispatchRepository interface {
	ListEnabled(context.Context, Event) ([]Rule, error)
	ReserveAndEnqueue(context.Context, Rule, Event) (QueueOutcome, error)
}

type QueueOutcome int

const (
	QueueCreated QueueOutcome = iota
	QueueDuplicate
	QueueRateLimited
)

type DispatchResult struct {
	Evaluated   int `json:"evaluated"`
	Matched     int `json:"matched"`
	Queued      int `json:"queued"`
	Duplicate   int `json:"duplicate"`
	RateLimited int `json:"rateLimited"`
}

// Dispatch evaluates enabled rules deterministically and uses repository-side
// atomic fences for the shared hourly rate bucket and execution/job creation.
// Rule condition failures are data errors and fail the job instead of silently
// treating a malformed rule as a match.
func Dispatch(ctx context.Context, repository DispatchRepository, event Event) (DispatchResult, error) {
	if repository == nil {
		return DispatchResult{}, errors.New("automation dispatch repository is required")
	}
	if !validTrigger(event.Trigger) {
		return DispatchResult{}, errors.New("automation event trigger is invalid")
	}
	if event.Depth >= MaxExecutionDepth {
		return DispatchResult{}, ErrRecursionLimit
	}
	rules, err := repository.ListEnabled(ctx, event)
	if err != nil {
		return DispatchResult{}, fmt.Errorf("list automation rules: %w", err)
	}
	result := DispatchResult{Evaluated: len(rules)}
	for _, rule := range rules {
		if err := ValidateRule(rule.Spec); err != nil {
			return result, fmt.Errorf("validate persisted rule %s: %w", rule.ID.String(), err)
		}
		matched, err := Evaluate(rule.Spec.Conditions, event)
		if err != nil {
			return result, fmt.Errorf("evaluate rule %s: %w", rule.ID.String(), err)
		}
		if !matched {
			continue
		}
		result.Matched++
		outcome, err := repository.ReserveAndEnqueue(ctx, rule, event)
		if err != nil {
			return result, fmt.Errorf("reserve automation execution: %w", err)
		}
		switch outcome {
		case QueueCreated:
			result.Queued++
		case QueueDuplicate:
			result.Duplicate++
		case QueueRateLimited:
			result.RateLimited++
		default:
			return result, errors.New("automation repository returned an invalid queue outcome")
		}
	}
	return result, nil
}
