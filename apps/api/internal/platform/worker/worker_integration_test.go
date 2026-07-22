//go:build integration

package worker

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestPostgresJobQueueConcurrencyAndLeaseFencing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adminURL, dispatcherURL := isolatedWorkerDatabase(t, ctx)
	admin, err := database.Open(ctx, adminURL, 2, "job-queue-integration-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	dispatcher, err := database.Open(ctx, dispatcherURL, 4, "job-queue-integration-dispatcher")
	if err != nil {
		t.Fatal(err)
	}
	defer dispatcher.Close()

	workspaceID := mustWorkerIntegrationID(t)
	if _, err := admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES($1,'Job queue integration',$2)`,
		workspaceID.PG(), "job-queue-"+compactWorkerID(workspaceID)[:24]); err != nil {
		t.Fatal(err)
	}
	queries := dbgen.New(dispatcher)

	t.Run("two claimers cannot own the same job", func(t *testing.T) {
		jobID := insertWorkerIntegrationJob(t, ctx, admin, workspaceID, 3)
		workers := []string{"claimer-a", "claimer-b"}
		start := make(chan struct{})
		results := make(chan workerClaimResult, len(workers))
		var group sync.WaitGroup
		for _, workerID := range workers {
			workerID := workerID
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				row, claimErr := queries.ClaimJob(ctx, dbgen.ClaimJobParams{
					LeaseMilliseconds: 2_000,
					WorkerID:          &workerID,
				})
				results <- workerClaimResult{row: row, err: claimErr}
			}()
		}
		close(start)
		group.Wait()
		close(results)

		var winner dbgen.ClaimJobRow
		successes, misses := 0, 0
		for result := range results {
			switch {
			case result.err == nil:
				successes++
				winner = result.row
			case errors.Is(result.err, pgx.ErrNoRows):
				misses++
			default:
				t.Fatalf("claim job: %v", result.err)
			}
		}
		if successes != 1 || misses != 1 {
			t.Fatalf("claim outcomes successes=%d misses=%d, want 1/1", successes, misses)
		}
		claimedID, ok := ids.FromPG(winner.ID)
		if !ok || claimedID != jobID || winner.Attempts != 1 || winner.FencingToken != 1 || winner.WorkerID == nil {
			t.Fatalf("unexpected winning claim: id=%v attempts=%d fencing=%d worker=%v",
				winner.ID, winner.Attempts, winner.FencingToken, winner.WorkerID)
		}
		if _, err := queries.CompleteJob(ctx, dbgen.CompleteJobParams{
			WorkspaceID: winner.WorkspaceID, ID: winner.ID, FencingToken: winner.FencingToken, WorkerID: winner.WorkerID,
		}); err != nil {
			t.Fatalf("complete winning claim: %v", err)
		}
	})

	t.Run("expired lease rejects stale completion and can be reclaimed", func(t *testing.T) {
		jobID := insertWorkerIntegrationJob(t, ctx, admin, workspaceID, 3)
		firstWorker := "lease-owner-a"
		first, err := queries.ClaimJob(ctx, dbgen.ClaimJobParams{LeaseMilliseconds: 120, WorkerID: &firstWorker})
		if err != nil {
			t.Fatal(err)
		}
		waitForWorkerLeaseExpiry(t, ctx, admin, workspaceID, jobID)
		if _, err := queries.CompleteJob(ctx, dbgen.CompleteJobParams{
			WorkspaceID: first.WorkspaceID, ID: first.ID, FencingToken: first.FencingToken, WorkerID: first.WorkerID,
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expired completion error=%v, want pgx.ErrNoRows", err)
		}

		secondWorker := "lease-owner-b"
		second, err := queries.ClaimJob(ctx, dbgen.ClaimJobParams{LeaseMilliseconds: 2_000, WorkerID: &secondWorker})
		if err != nil {
			t.Fatal(err)
		}
		secondID, ok := ids.FromPG(second.ID)
		if !ok || secondID != jobID || second.Attempts != 2 || second.FencingToken != 2 {
			t.Fatalf("reclaimed job id=%v attempts=%d fencing=%d", second.ID, second.Attempts, second.FencingToken)
		}
		if _, err := queries.CompleteJob(ctx, dbgen.CompleteJobParams{
			WorkspaceID: first.WorkspaceID, ID: first.ID, FencingToken: first.FencingToken, WorkerID: first.WorkerID,
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("stale completion error=%v, want pgx.ErrNoRows", err)
		}
		state, err := queries.CompleteJob(ctx, dbgen.CompleteJobParams{
			WorkspaceID: second.WorkspaceID, ID: second.ID, FencingToken: second.FencingToken, WorkerID: second.WorkerID,
		})
		if err != nil || state != "completed" {
			t.Fatalf("current owner completion state=%q err=%v", state, err)
		}
	})

	t.Run("failures retry and reach dead letter state", func(t *testing.T) {
		jobID := insertWorkerIntegrationJob(t, ctx, admin, workspaceID, 2)
		firstWorker := "retry-owner-a"
		first, err := queries.ClaimJob(ctx, dbgen.ClaimJobParams{LeaseMilliseconds: 2_000, WorkerID: &firstWorker})
		if err != nil {
			t.Fatal(err)
		}
		firstCode := "first_failure"
		failed, err := queries.FailJob(ctx, dbgen.FailJobParams{
			DelayMilliseconds: 0, ErrorCode: &firstCode, WorkspaceID: first.WorkspaceID,
			ID: first.ID, WorkerID: first.WorkerID, FencingToken: first.FencingToken,
		})
		if err != nil || failed.State != "ready" || failed.Attempts != 1 {
			t.Fatalf("first failure state=%q attempts=%d err=%v", failed.State, failed.Attempts, err)
		}

		secondWorker := "retry-owner-b"
		second, err := queries.ClaimJob(ctx, dbgen.ClaimJobParams{LeaseMilliseconds: 2_000, WorkerID: &secondWorker})
		if err != nil {
			t.Fatal(err)
		}
		secondCode := "final_failure"
		failed, err = queries.FailJob(ctx, dbgen.FailJobParams{
			DelayMilliseconds: 0, ErrorCode: &secondCode, WorkspaceID: second.WorkspaceID,
			ID: second.ID, WorkerID: second.WorkerID, FencingToken: second.FencingToken,
		})
		if err != nil || failed.State != "dead" || failed.Attempts != 2 {
			t.Fatalf("final failure state=%q attempts=%d err=%v", failed.State, failed.Attempts, err)
		}
		var state, errorCode string
		var attempts int
		if err := admin.QueryRow(ctx, `SELECT state,attempts,last_error_code FROM platform.jobs WHERE workspace_id=$1 AND id=$2`,
			workspaceID.PG(), jobID.PG()).Scan(&state, &attempts, &errorCode); err != nil {
			t.Fatal(err)
		}
		if state != "dead" || attempts != 2 || errorCode != secondCode {
			t.Fatalf("persisted dead letter state=%q attempts=%d error=%q", state, attempts, errorCode)
		}
		if _, err := queries.ClaimJob(ctx, dbgen.ClaimJobParams{LeaseMilliseconds: 2_000, WorkerID: &secondWorker}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("dead job was claimable: %v", err)
		}
	})

	t.Run("expired final attempt is moved to dead letter state", func(t *testing.T) {
		jobID := insertWorkerIntegrationJob(t, ctx, admin, workspaceID, 1)
		workerID := "abandoned-owner"
		if _, err := queries.ClaimJob(ctx, dbgen.ClaimJobParams{LeaseMilliseconds: 120, WorkerID: &workerID}); err != nil {
			t.Fatal(err)
		}
		waitForWorkerLeaseExpiry(t, ctx, admin, workspaceID, jobID)
		affected, err := queries.MarkExhaustedJobsDead(ctx)
		if err != nil || affected != 1 {
			t.Fatalf("mark exhausted affected=%d err=%v", affected, err)
		}
		var state, errorCode string
		var persistedWorker *string
		if err := admin.QueryRow(ctx, `SELECT state,last_error_code,worker_id FROM platform.jobs WHERE workspace_id=$1 AND id=$2`,
			workspaceID.PG(), jobID.PG()).Scan(&state, &errorCode, &persistedWorker); err != nil {
			t.Fatal(err)
		}
		if state != "dead" || errorCode != "lease_expired" || persistedWorker != nil {
			t.Fatalf("expired final attempt state=%q error=%q worker=%v", state, errorCode, persistedWorker)
		}
	})
}

type workerClaimResult struct {
	row dbgen.ClaimJobRow
	err error
}

func insertWorkerIntegrationJob(
	t *testing.T, ctx context.Context, admin *pgxpool.Pool, workspaceID ids.UUID, maxAttempts int,
) ids.UUID {
	t.Helper()
	jobID := mustWorkerIntegrationID(t)
	if _, err := admin.Exec(ctx, `
INSERT INTO platform.jobs(workspace_id,id,kind,schema_version,idempotency_key,payload,max_attempts)
VALUES($1,$2,'integration.queue',1,$3,'{}'::jsonb,$4)`,
		workspaceID.PG(), jobID.PG(), jobID.String(), maxAttempts); err != nil {
		t.Fatal(err)
	}
	return jobID
}

func waitForWorkerLeaseExpiry(
	t *testing.T, ctx context.Context, admin *pgxpool.Pool, workspaceID, jobID ids.UUID,
) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var expired bool
		if err := admin.QueryRow(ctx, `
SELECT locked_until IS NOT NULL AND locked_until <= clock_timestamp()
FROM platform.jobs WHERE workspace_id=$1 AND id=$2`, workspaceID.PG(), jobID.PG()).Scan(&expired); err != nil {
			t.Fatal(err)
		}
		if expired {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job lease did not expire before deadline")
}

func isolatedWorkerDatabase(t *testing.T, ctx context.Context) (string, string) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	coordinator, err := database.Open(ctx, adminURL, 1, "job-queue-test-coordinator")
	if err != nil {
		t.Fatal(err)
	}
	databaseID := mustWorkerIntegrationID(t)
	databaseName := "veltrix_worker_it_" + compactWorkerID(databaseID)[:20]
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := coordinator.Exec(ctx, "CREATE DATABASE "+quotedName); err != nil {
		coordinator.Close()
		t.Fatalf("create isolated database: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := coordinator.Exec(cleanupCtx, "DROP DATABASE "+quotedName+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated database: %v", err)
		}
		coordinator.Close()
	})

	testAdminURL := workerDatabaseURL(t, adminURL, databaseName, "", "")
	if err := database.Migrate(ctx, testAdminURL, appPassword); err != nil {
		t.Fatalf("migrate isolated database: %v", err)
	}
	testDispatcherURL := workerDatabaseURL(t, appURL, databaseName, "veltrix_dispatcher", appPassword)
	return testAdminURL, testDispatcherURL
}

func workerDatabaseURL(t *testing.T, rawURL, databaseName, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	parsed.Path = "/" + databaseName
	if username != "" {
		parsed.User = url.UserPassword(username, password)
	}
	return parsed.String()
}

func mustWorkerIntegrationID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func compactWorkerID(id ids.UUID) string {
	return strings.ReplaceAll(id.String(), "-", "")
}
