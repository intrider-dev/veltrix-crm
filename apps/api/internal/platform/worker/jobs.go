package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func (runtime *runtime) processOneJob(ctx context.Context) (bool, error) {
	operationCtx, cancel := context.WithTimeout(ctx, runtime.config.OperationTimeout)
	row, err := dbgen.New(runtime.config.DispatcherPool).ClaimJob(operationCtx, dbgen.ClaimJobParams{
		LeaseMilliseconds: runtime.config.LeaseDuration.Milliseconds(),
		WorkerID:          &runtime.config.WorkerID,
	})
	cancel()
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim job: %w", err)
	}
	job, err := claimedJob(row, runtime.config.WorkerID)
	if err != nil {
		return true, err
	}
	if !job.leaseValid(time.Now().UTC()) {
		return true, ErrLeaseLost
	}

	handler, exists := runtime.config.Handlers[job.Kind]
	if !exists {
		handler = func(context.Context, Dependencies, Job) error {
			return unknownJobKindError{kind: job.Kind}
		}
	}
	deadline := time.Now().UTC().Add(runtime.config.JobTimeout)
	if job.LockedUntil.Before(deadline) {
		deadline = job.LockedUntil
	}
	handlerCtx, stopHandler := context.WithDeadline(ctx, deadline)
	handlerErr := handler(handlerCtx, Dependencies{AppPool: runtime.config.AppPool}, job)
	if handlerErr == nil && handlerCtx.Err() != nil {
		handlerErr = handlerCtx.Err()
	}
	stopHandler()
	if ctx.Err() != nil {
		// Cancellation deliberately leaves the job leased. A later worker can
		// reclaim it after locked_until without accepting a stale completion.
		return true, nil
	}
	if handlerErr == nil {
		return true, runtime.completeJob(ctx, job)
	}
	return true, runtime.failJob(ctx, job, failureCode(handlerErr))
}

func claimedJob(row dbgen.ClaimJobRow, expectedWorkerID string) (Job, error) {
	workspaceID, workspaceValid := ids.FromPG(row.WorkspaceID)
	jobID, jobValid := ids.FromPG(row.ID)
	if !workspaceValid || !jobValid || !row.LockedAt.Valid || !row.LockedUntil.Valid || row.WorkerID == nil {
		return Job{}, errors.New("claimed job contains invalid lease fields")
	}
	if *row.WorkerID != expectedWorkerID || !row.LockedAt.Time.Before(row.LockedUntil.Time) {
		return Job{}, errors.New("claimed job contains inconsistent lease ownership")
	}
	return Job{
		WorkspaceID:    workspaceID,
		ID:             jobID,
		Kind:           row.Kind,
		SchemaVersion:  row.SchemaVersion,
		IdempotencyKey: row.IdempotencyKey,
		Payload:        append([]byte(nil), row.Payload...),
		Attempts:       row.Attempts,
		MaxAttempts:    row.MaxAttempts,
		LockedAt:       row.LockedAt.Time.UTC(),
		LockedUntil:    row.LockedUntil.Time.UTC(),
		FencingToken:   row.FencingToken,
	}, nil
}

func (runtime *runtime) completeJob(ctx context.Context, job Job) error {
	operationCtx, cancel := context.WithTimeout(ctx, runtime.config.OperationTimeout)
	defer cancel()
	state, err := dbgen.New(runtime.config.DispatcherPool).CompleteJob(operationCtx, dbgen.CompleteJobParams{
		WorkspaceID:  job.WorkspaceID.PG(),
		ID:           job.ID.PG(),
		FencingToken: job.FencingToken,
		WorkerID:     &runtime.config.WorkerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	if state != "completed" {
		return fmt.Errorf("complete job returned unexpected state %q", state)
	}
	return nil
}

func (runtime *runtime) failJob(ctx context.Context, job Job, errorCode string) error {
	if len(errorCode) > 120 || errorCode == "" {
		errorCode = "handler_failed"
	}
	delay := Backoff(job.Attempts, runtime.config.BackoffBase, runtime.config.BackoffMaximum)
	operationCtx, cancel := context.WithTimeout(ctx, runtime.config.OperationTimeout)
	defer cancel()
	result, err := dbgen.New(runtime.config.DispatcherPool).FailJob(operationCtx, dbgen.FailJobParams{
		DelayMilliseconds: delay.Milliseconds(),
		ErrorCode:         &errorCode,
		WorkspaceID:       job.WorkspaceID.PG(),
		ID:                job.ID.PG(),
		WorkerID:          &runtime.config.WorkerID,
		FencingToken:      job.FencingToken,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return fmt.Errorf("fail job: %w", err)
	}
	if result.State != "ready" && result.State != "dead" {
		return fmt.Errorf("fail job returned unexpected state %q", result.State)
	}
	runtime.config.Logger.Warn("worker job failed",
		"component", "job",
		"job_id", job.ID.String(),
		"kind", job.Kind,
		"attempt", result.Attempts,
		"max_attempts", result.MaxAttempts,
		"state", result.State,
		"error_code", errorCode,
	)
	return nil
}
