-- name: ClaimOutboxEvents :many
SELECT workspace_id, id, event_type, schema_version, aggregate_type,
       aggregate_id, correlation_id, payload, attempts
FROM platform.outbox_events
WHERE published_at IS NULL
  AND available_at <= now()
ORDER BY available_at, created_at, id
LIMIT sqlc.arg(batch_size)
FOR UPDATE SKIP LOCKED;

-- name: InsertFanoutJob :exec
INSERT INTO platform.jobs (
  workspace_id, id, kind, schema_version, idempotency_key, payload,
  state, attempts, max_attempts, available_at
)
VALUES ($1, $2, $3, $4, $5, $6, 'ready', 0, $7, now())
ON CONFLICT (workspace_id, kind, idempotency_key) DO NOTHING;

-- name: MarkOutboxPublished :execrows
UPDATE platform.outbox_events
SET published_at = now(),
    last_error_code = NULL
WHERE workspace_id = $1
  AND id = $2
  AND published_at IS NULL;

-- name: RecordOutboxFailure :exec
UPDATE platform.outbox_events
SET attempts = attempts + 1,
    available_at = now() + (sqlc.arg(delay_milliseconds)::bigint * interval '1 millisecond'),
    last_error_code = sqlc.arg(error_code)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND published_at IS NULL;

-- name: ClaimJob :one
WITH candidate AS (
  SELECT workspace_id, id
  FROM platform.jobs
  WHERE attempts < max_attempts
    AND (
      (state = 'ready' AND available_at <= now())
      OR (state = 'running' AND locked_until <= now())
  )
  ORDER BY available_at, created_at, id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
UPDATE platform.jobs AS job
SET state = 'running',
    attempts = job.attempts + 1,
    locked_at = now(),
    locked_until = now() + (sqlc.arg(lease_milliseconds)::bigint * interval '1 millisecond'),
    worker_id = sqlc.arg(worker_id),
    fencing_token = job.fencing_token + 1,
    last_error_code = NULL,
    updated_at = now()
FROM candidate
WHERE job.workspace_id = candidate.workspace_id
  AND job.id = candidate.id
RETURNING job.workspace_id, job.id, job.kind, job.schema_version,
          job.idempotency_key, job.payload, job.attempts, job.max_attempts,
          job.locked_at, job.locked_until, job.worker_id, job.fencing_token;

-- name: CompleteJob :one
UPDATE platform.jobs
SET state = 'completed',
    completed_at = COALESCE(completed_at, now()),
    locked_at = NULL,
    locked_until = NULL,
    worker_id = NULL,
    last_error_code = NULL,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND fencing_token = sqlc.arg(fencing_token)
  AND (
    (
      state = 'running'
      AND worker_id = sqlc.arg(worker_id)
      AND locked_until > now()
    )
    OR state = 'completed'
  )
RETURNING state;

-- name: FailJob :one
UPDATE platform.jobs
SET state = CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'ready' END,
    available_at = CASE
      WHEN attempts >= max_attempts THEN available_at
      ELSE now() + (sqlc.arg(delay_milliseconds)::bigint * interval '1 millisecond')
    END,
    locked_at = NULL,
    locked_until = NULL,
    worker_id = NULL,
    last_error_code = sqlc.arg(error_code),
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND state = 'running'
  AND worker_id = sqlc.arg(worker_id)
  AND fencing_token = sqlc.arg(fencing_token)
  AND locked_until > now()
RETURNING state, attempts, max_attempts;

-- name: MarkExhaustedJobsDead :execrows
WITH exhausted AS (
  SELECT workspace_id, id
  FROM platform.jobs
  WHERE state = 'running'
    AND locked_until <= now()
    AND attempts >= max_attempts
  ORDER BY locked_until, id
  LIMIT 500
  FOR UPDATE SKIP LOCKED
)
UPDATE platform.jobs AS job
SET state = 'dead',
    locked_at = NULL,
    locked_until = NULL,
    worker_id = NULL,
    last_error_code = COALESCE(last_error_code, 'lease_expired'),
    updated_at = now()
FROM exhausted
WHERE job.workspace_id = exhausted.workspace_id
  AND job.id = exhausted.id;
