# ADR 0003: Tenant isolation with application guards and forced RLS

- Status: Accepted
- Date: 2026-07-21

## Context

Every CRM object belongs to a workspace. A missing tenant predicate or object authorization check must not expose another workspace through CRUD, search, export, reporting, jobs, or SSE. Pooled database connections make session-level tenant variables unsafe.

## Decision

Use two mandatory layers:

1. HTTP/service guards authenticate the actor, resolve membership, check role/permission and object ownership, and reject a workspace supplied only by untrusted input.
2. Every tenant-owned table carries `workspace_id`, enables and forces PostgreSQL RLS, and has a policy bound to `security.current_workspace_id()`.

Runtime transactions use the non-superuser, `NOBYPASSRLS` `veltrix_app` role. Immediately after `BEGIN`, the application sets actor and workspace context with `SET LOCAL`; no tenant query may run outside this wrapper. Commit/rollback clears context before the connection returns to the pool.

Schema ownership belongs to `veltrix_owner`, a `NOLOGIN` role assumed only by the migrator. Cross-tenant outbox/job claiming uses `veltrix_dispatcher` with narrow explicit grants and is not available to request handlers.

Real-PostgreSQL negative tests enumerate tenant tables/policies and prove denial of cross-tenant read, search, insert, update, delete, export, SSE replay, and pooled-connection leakage.

## Consequences

- An omitted SQL tenant predicate is still denied by RLS when the role/context invariant holds.
- Transaction setup and database grants become security-critical code and require focused review.
- Owners/superusers can bypass RLS, so admin credentials must not serve application requests.
- Every new tenant table and dispatcher grant must extend migration invariants and negative tests.
- Background system work must re-establish an explicit workspace transaction before reading tenant payload data.

## Alternatives rejected

- **Application filters only:** one missed path becomes a data breach.
- **Schema/database per tenant:** stronger physical separation but disproportionate migration/pool/operational cost for the target tier.
- **Session-level `SET`:** context can leak through pooled connections.
- **A single privileged DB role:** broadens the impact of an application defect and defeats defense in depth.
