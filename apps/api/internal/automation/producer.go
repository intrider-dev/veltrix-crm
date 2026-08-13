package automation

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultProducerInterval = time.Minute

// RunTriggerProducer invokes the narrow SECURITY DEFINER database boundary
// that creates idempotent hourly scheduled events and overdue-task events.
// Multiple server/worker processes may run it concurrently; PostgreSQL ledger
// constraints make a tick at-most-once before the transactional outbox.
func RunTriggerProducer(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, interval time.Duration) {
	if pool == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = defaultProducerInterval
	}
	produceTriggerEvents(ctx, pool, logger)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			produceTriggerEvents(ctx, pool, logger)
		}
	}
}

func produceTriggerEvents(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) {
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var scheduledCount, overdueCount int
	err := pool.QueryRow(operationCtx,
		"SELECT scheduled_count, overdue_count FROM automation.enqueue_due_trigger_events()",
	).Scan(&scheduledCount, &overdueCount)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error("automation trigger producer failed",
				"component", "automation_producer", "error_code", "enqueue_failed")
		}
		return
	}
	if scheduledCount > 0 || overdueCount > 0 {
		logger.Info("automation trigger events queued", "component", "automation_producer",
			"scheduled", scheduledCount, "overdue", overdueCount)
	}
}
