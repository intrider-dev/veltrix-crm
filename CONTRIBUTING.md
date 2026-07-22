# Contributing

Thank you for helping build a resource-conscious, multi-tenant Sales CRM. Contributions should be focused, testable, accessible, and explicit about security and performance trade-offs.

## Before starting

For a small bug, open or reference an issue and keep the patch narrow. For a schema, API, architecture, dependency, or user-flow change, start a discussion or design issue before investing in a large implementation. Security vulnerabilities follow `SECURITY.md`, not the public issue tracker.

Read `AGENTS.md`, `docs/MASTER_PROMPT.md`, `docs/STATE.md`, and relevant ADRs. Existing files and user work must be preserved.

## Local setup

Requirements are a recent Docker/Compose installation for the production-like path, or the exact Node.js, pnpm, Go, and PostgreSQL versions pinned by the repository for native development.

```sh
cp .env.example .env
corepack pnpm install --frozen-lockfile
corepack pnpm generate:contracts
docker compose up --build
```

Use synthetic data only. The documented demo credentials are for local development and must never be enabled in production.

## Canonical checks

Run the smallest relevant checks while iterating, then the complete applicable gate before requesting review:

```sh
corepack pnpm lint
corepack pnpm typecheck
corepack pnpm test
corepack pnpm test:integration
corepack pnpm test:e2e
corepack pnpm build
```

Database, E2E, Docker, Lighthouse, security, and benchmark checks require their documented services and tools. If a check cannot run, state the exact command, failure reason, and remaining uncertainty in the pull request; never report it as passed.

## Architecture boundaries

- Keep the Go backend a modular monolith; modules communicate through explicit interfaces rather than reaching into each other’s internal tables.
- Every tenant-owned table and index follows the workspace-first/RLS rules. Add negative cross-tenant tests for every new access path.
- API contracts originate in OpenAPI. Run contract generation and commit deterministic generated output.
- Large lists use cursor pagination; mutations that can duplicate use idempotency keys; concurrent edits use versions or ETags.
- Keep runtime queues, caches, connections, uploads, request bodies, SSE replay, and worker concurrency bounded.
- Do not add Redis, a broker, or a search cluster to the required base profile.

## Frontend and localization

Use standalone zoneless Angular, signals, OnPush change detection, route-level lazy loading, Material/CDK/Aria, and accessible keyboard/focus behavior. Do not add Tailwind, an external font, a second UI kit, `ag-grid-enterprise`, Moment.js, full Lodash, or browser-stored auth tokens.

English is the source locale and Russian must remain complete. Every user-visible string uses a typed catalog key. Add or change messages in the source catalogs, update all required locales, preserve placeholders, regenerate catalogs, and run `corepack pnpm check:i18n`. User-authored CRM content is not silently translated.

## Tests and evidence

Tests should fail for the regression before the fix where practical. Include unit tests plus integration/E2E coverage proportional to risk. Tenant isolation, authorization, idempotency, retries, and webhook signatures require negative cases.

Performance changes need comparable methodology, raw artifacts, multiple runs, and median/p95/p99 where applicable. Do not publish a best single run or compare named competitors without authorized, reproducible measurements.

## Pull requests

Use a Conventional Commit-style title, keep generated and unrelated formatting changes out of the patch, complete the pull request template, and make reviewable commits. By contributing, you agree that your contribution is licensed under the repository’s MIT license.
