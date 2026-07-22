# VeltrixCRM agent guide

This repository is a production-oriented, open-source, multi-tenant Sales CRM. Preserve existing user files and keep claims evidence-based.

## Non-negotiable architecture

- Angular 22 SPA: standalone, strict TypeScript, zoneless, signals, Angular Material/CDK/Aria, lazy feature routes.
- Go modular monolith: `net/http`, `chi`, `pgx`, generated/type-safe SQL, OpenAPI 3.1.
- PostgreSQL 18 is the only required stateful infrastructure. No Redis, broker, or search cluster in the base profile.
- One production Go binary serves the API, SSE, workers, and embedded precompressed SPA assets.
- Tenant isolation is enforced by application guards and PostgreSQL RLS with transaction-local tenant context.
- API errors use stable machine-readable codes; UI text, notification templates, and validation messages are localized separately.
- Brand strings come from central brand configuration. Never scatter `VeltrixCRM` through source files.
- Do not add `ag-grid-enterprise`, external fonts, Tailwind, Moment.js, full Lodash, or auth tokens in local storage.

## Working rules

1. Read `docs/MASTER_PROMPT.md`, `docs/STATE.md`, and relevant ADRs before changing architecture.
2. Complete vertical slices; do not create empty feature shells just to imply coverage.
3. Store timestamps in UTC, money in integer minor units plus ISO currency, and tenant-owned data with `workspace_id` first in compound indexes.
4. Use parameterized SQL, cursor pagination for large lists, optimistic concurrency (`version`/ETag), and idempotency keys for duplicate-prone mutations.
5. Keep caches, worker concurrency, request sizes, uploads, SSE replay, and queues bounded.
6. Never invent benchmark results, screenshots, security checks, integrations, or test outcomes.
7. Every user-visible string must use the i18n system. English is the source locale; Russian must remain complete. CI rejects missing or stale keys.
8. After a material phase run formatter, lint, typecheck, tests, inspect `git diff`, and update `docs/STATE.md`.

## Expected commands

The root `pnpm` interface is canonical: `bootstrap`, `dev`, `build`, `lint`, `typecheck`, `test`, `test:integration`, `test:e2e`, `seed:small`, `seed:benchmark`, `benchmark`, and `check`. `docker compose up --build` is the production-like path.

## Documentation truthfulness

- Put actual measured values in `docs/PERFORMANCE.md`; use `Not measured` until a command has run successfully.
- Document unavailable tools with the failed command, reason, and remaining uncertainty.
- Keep `docs/STATE.md` current and newest changelog entries first.

