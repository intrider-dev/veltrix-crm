package worker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBackoffIsExponentialAndCapped(t *testing.T) {
	t.Parallel()

	base := 250 * time.Millisecond
	maximum := 2 * time.Second
	tests := []struct {
		attempt int32
		want    time.Duration
	}{
		{0, base},
		{1, base},
		{2, 500 * time.Millisecond},
		{3, time.Second},
		{4, maximum},
		{30, maximum},
	}
	for _, test := range tests {
		if got := Backoff(test.attempt, base, maximum); got != test.want {
			t.Errorf("Backoff(%d) = %s, want %s", test.attempt, got, test.want)
		}
	}
}

func TestLeaseValidityUsesExclusiveDeadline(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 21, 10, 0, 0, 0, time.UTC)
	job := Job{LockedUntil: now.Add(time.Second)}
	if !job.leaseValid(now) {
		t.Fatal("fresh lease reported invalid")
	}
	if job.leaseValid(job.LockedUntil) {
		t.Fatal("lease remained valid at its exclusive deadline")
	}
	if job.leaseValid(job.LockedUntil.Add(time.Nanosecond)) {
		t.Fatal("expired lease reported valid")
	}
}

func TestFanoutKindsAreRelevantAndStable(t *testing.T) {
	t.Parallel()

	contact := fanoutKinds("customers.contact.updated", "contact", nil)
	assertKinds(t, contact, "search.sync", "automation.dispatch", "webhook.dispatch")
	dealMove := fanoutKinds("sales.deal.stage_changed", "deal", nil)
	assertKinds(t, dealMove, "search.sync", "automation.dispatch", "notification.dispatch", "webhook.dispatch")
	activity := fanoutKinds("activities.activity.completed", "activity", nil)
	assertKinds(t, activity, "automation.dispatch", "notification.dispatch", "webhook.dispatch")
	note := fanoutKinds("activities.activity.created", "activity", []byte(`{"type":"note"}`))
	assertKinds(t, note, "search.sync", "automation.dispatch", "notification.dispatch", "webhook.dispatch")
}

func TestFailureCodesDoNotExposeArbitraryErrors(t *testing.T) {
	t.Parallel()

	if got := failureCode(errors.New("customer email and token")); got != "handler_failed" {
		t.Fatalf("failureCode() = %q, want handler_failed", got)
	}
	if got := failureCode(context.DeadlineExceeded); got != "job_timeout" {
		t.Fatalf("deadline failure code = %q, want job_timeout", got)
	}
	if got := failureCode(unknownJobKindError{kind: "future.kind"}); got != "unknown_job_kind" {
		t.Fatalf("unknown job failure code = %q", got)
	}
}

func TestConfigDefaultsStayBounded(t *testing.T) {
	t.Parallel()

	config, err := (Config{DispatcherPool: &pgxpool.Pool{}, WorkerID: "test-worker"}).normalized()
	if err != nil {
		t.Fatalf("normalized() error = %v", err)
	}
	if config.Concurrency != 2 || config.Concurrency > maxWorkerConcurrency {
		t.Fatalf("default concurrency = %d, want 2 and bounded", config.Concurrency)
	}
	if config.JobTimeout >= config.LeaseDuration {
		t.Fatalf("job timeout %s is not shorter than lease %s", config.JobTimeout, config.LeaseDuration)
	}
	if config.OutboxBatchSize != 50 || config.MaxAttempts != 8 {
		t.Fatalf("unexpected queue defaults: batch=%d attempts=%d", config.OutboxBatchSize, config.MaxAttempts)
	}
}

func TestRunReturnsAfterPreCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, Config{DispatcherPool: &pgxpool.Pool{}, WorkerID: "test-worker"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestWorkerSQLContainsConcurrencyAndFencingGuards(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../../../queries/worker.sql")
	if err != nil {
		t.Fatalf("read worker SQL: %v", err)
	}
	sql := strings.ToLower(string(contents))
	checks := []string{
		"for update skip locked",
		"fencing_token = job.fencing_token + 1",
		"fencing_token = sqlc.arg(fencing_token)",
		"locked_until > now()",
		"or state = 'completed'",
		"attempts >= max_attempts then 'dead'",
		"on conflict (workspace_id, kind, idempotency_key) do nothing",
		"limit 500",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Errorf("worker SQL is missing %q", check)
		}
	}
	if count := strings.Count(sql, "for update skip locked"); count < 2 {
		t.Errorf("worker SQL contains %d SKIP LOCKED claims, want at least 2", count)
	}
}

func assertKinds(t *testing.T, actual []string, expected ...string) {
	t.Helper()
	if strings.Join(actual, ",") != strings.Join(expected, ",") {
		t.Fatalf("fanout kinds = %v, want %v", actual, expected)
	}
}
