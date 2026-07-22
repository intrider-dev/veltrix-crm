# ADR 0004: PostgreSQL transactional outbox, job queue, and SSE replay

- Status: Accepted
- Date: 2026-07-21

## Context

Search indexing, notifications, automations, email, and webhooks happen after domain mutations but must not be silently lost. The base deployment may not require Redis, Kafka, RabbitMQ, or a workflow service. Workers can crash, retry, overlap, or lose leases; browsers can disconnect and reconnect.

## Decision

- Write domain state and an outbox event in one PostgreSQL transaction.
- Let a bounded dispatcher claim available outbox rows and create durable, idempotent jobs.
- Claim jobs with `SELECT ... FOR UPDATE SKIP LOCKED`, worker identity, `locked_at`/lease deadline, attempt count, `available_at`, handler timeout, exponential backoff, maximum attempts, and dead state.
- Assume at-least-once execution. Every handler and external action needs an idempotency fence/key; a lease is coordination, not exactly-once delivery.
- Run bounded concurrency inside the default server, while exposing the same runtime through a separate `worker` command for future horizontal scaling.
- Persist tenant-scoped SSE events for bounded `Last-Event-ID` replay and use a bounded in-memory hub only for live delivery. Authorize the endpoint, send heartbeats, remove cancelled clients, and disconnect/skip rather than grow an unbounded slow-client queue.

## Consequences

- Domain durability and asynchronous intent are atomic without distributed transactions.
- Queue/search work shares PostgreSQL CPU, WAL, locks, and storage with CRUD; benchmark and backlog monitoring are required.
- External effects are not exactly once; webhook/email/action implementations must tolerate retry.
- Poison work becomes visible in failed/dead records and needs operational inspection/retry controls.
- Cross-tenant dispatcher access is a narrow high-trust boundary covered by explicit grants and review.

## Alternatives rejected

- **Publish directly after commit:** a process crash can lose the effect.
- **Publish before commit:** consumers can observe rolled-back state.
- **In-memory queue:** loses work on restart and cannot support controlled retries.
- **Required external broker:** raises the operational floor before measurements justify it.
- **Notification polling:** wastes requests/CPU and delays delivery when SSE is available.
