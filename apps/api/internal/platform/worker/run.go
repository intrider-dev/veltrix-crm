package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
)

type runtime struct {
	config Config
}

func Run(ctx context.Context, config Config) error {
	normalized, err := config.normalized()
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	runtime := &runtime{config: normalized}
	var waitGroup sync.WaitGroup

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		runtime.runOutboxLoop(ctx)
	}()
	for index := 0; index < normalized.Concurrency; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			runtime.runJobLoop(ctx)
		}()
	}

	<-ctx.Done()
	waitGroup.Wait()
	return nil
}

func (runtime *runtime) runOutboxLoop(ctx context.Context) {
	nextCleanup := time.Time{}
	for ctx.Err() == nil {
		operationCtx, cancel := context.WithTimeout(ctx, runtime.config.OperationTimeout)
		count, failure, err := runtime.dispatchOutboxBatch(operationCtx)
		cancel()
		if err != nil {
			runtime.recordOutboxFailure(ctx, failure)
			runtime.config.Logger.Error("worker outbox dispatch failed",
				"component", "outbox",
				"error_code", "dispatch_failed",
			)
			if !waitFor(ctx, runtime.config.PollInterval) {
				return
			}
			continue
		}
		if now := time.Now().UTC(); !now.Before(nextCleanup) {
			runtime.cleanupExhaustedJobs(ctx)
			nextCleanup = now.Add(runtime.config.LeaseDuration)
		}
		if int32(count) < runtime.config.OutboxBatchSize && !waitFor(ctx, runtime.config.PollInterval) {
			return
		}
	}
}

func (runtime *runtime) runJobLoop(ctx context.Context) {
	for ctx.Err() == nil {
		processed, err := runtime.processOneJob(ctx)
		if err != nil && ctx.Err() == nil {
			code := "process_failed"
			if errors.Is(err, ErrLeaseLost) {
				code = "lease_lost"
			}
			runtime.config.Logger.Error("worker job processing failed", "component", "job", "error_code", code)
		}
		if (!processed || err != nil) && !waitFor(ctx, runtime.config.PollInterval) {
			return
		}
	}
}

func (runtime *runtime) cleanupExhaustedJobs(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, runtime.config.OperationTimeout)
	defer cancel()
	dead, err := dbgen.New(runtime.config.DispatcherPool).MarkExhaustedJobsDead(cleanupCtx)
	if err != nil && ctx.Err() == nil {
		runtime.config.Logger.Error("worker exhausted-job cleanup failed",
			"component", "job",
			"error_code", "cleanup_failed",
		)
		return
	}
	if dead > 0 {
		runtime.config.Logger.Warn("worker marked exhausted jobs dead", "component", "job", "count", dead)
	}
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
