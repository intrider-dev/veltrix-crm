# ADR 0001: Modular monolith and two-container base runtime

- Status: Accepted
- Date: 2026-07-21

## Context

The CRM needs substantial domain breadth, strict tenant boundaries, low idle resource usage, and a production-like setup that a portfolio reviewer can reproduce.

## Decision

Use a Go modular monolith and an Angular SPA. A single production Go binary serves `/api/v1`, tenant-scoped SSE, bounded background workers, and embedded precompressed SPA assets. PostgreSQL 18 provides durable relational storage, full-text/trigram search, jobs, outbox, automation logs, and replay data. The required production profile contains only the app and database containers.

## Consequences

- Transactions can atomically update domain state, audit, search/outbox records, and jobs.
- Deployment and local reproduction remain small.
- Module ownership must be enforced in packages and SQL review because a shared database does not create physical service boundaries.
- Independent worker mode remains a command of the same binary for future scaling.
