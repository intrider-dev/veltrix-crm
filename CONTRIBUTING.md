# Contributing to VeltrixCRM

VeltrixCRM is in active development. Focused bug fixes, tests, documentation improvements, and complete feature slices are welcome. Large changes should begin with a design discussion so that database, API, interface, localization, security, and performance work stay aligned.

## Before you start

- Read [docs/STATE.md](docs/STATE.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [DESIGN.md](DESIGN.md), and the relevant ADRs.
- Use the public issue tracker for defects and proposals. Report security problems privately as described in [SECURITY.md](SECURITY.md).
- Keep a change narrow. Preserve unrelated files and existing user work.
- Do not include real customer data, credentials, or private logs.

## Local setup

For the production-like path, install Docker with Compose v2:

```sh
cp .env.example .env
docker compose up --build
```

For native development, use the Node.js, pnpm, Go, and PostgreSQL versions pinned in the repository:

```sh
corepack pnpm install --frozen-lockfile
corepack pnpm generate:contracts
```

The documented demo credentials are for local development only. Never enable the demo seed in production.

## Development rules

### Backend and data

- Keep the Go backend a modular monolith. Modules communicate through explicit interfaces.
- Every tenant-owned access path needs an application guard and a forced-RLS path with transaction-local workspace context.
- Start workspace indexes with `workspace_id`; use parameterized SQL and cursor pagination for large collections.
- Generate shared types from OpenAPI. Do not maintain a second handwritten contract.
- Use idempotency keys for duplicate-prone mutations and versions or ETags for concurrent edits.
- Bound request bodies, uploads, queues, caches, connections, worker concurrency, and SSE replay.

### Frontend and content

- Use standalone zoneless Angular, strict TypeScript, signals, OnPush change detection, and lazy routes.
- Use the shared Material/CDK/Aria component layer and the rules in [DESIGN.md](DESIGN.md).
- Keep authentication tokens out of browser storage. Do not add external fonts, Tailwind, a second general UI kit, `ag-grid-enterprise`, Moment.js, or full Lodash.
- Put every visible string in the typed localization catalog. English is the source locale; Russian must remain complete.
- Preserve placeholders and plural forms. User-authored CRM content is translated only through the explicit workspace translation flow.

## Tests

Run the smallest relevant checks while working, then the complete applicable gate before requesting review:

```sh
corepack pnpm lint
corepack pnpm typecheck
corepack pnpm test
corepack pnpm test:integration
corepack pnpm test:e2e
corepack pnpm build
```

Add regression coverage where practical. Authorization, tenant isolation, idempotency, retries, uploads, and webhook signatures require negative cases. Interface changes should cover keyboard use, focus, localization, error states, and narrow viewports.

If a check cannot run, record the exact command, failure reason, and remaining uncertainty. Do not describe an unexecuted check as successful.

## Performance evidence

Performance changes need comparable inputs, fixed limits, raw artifacts, multiple runs, and median/p95/p99 where applicable. Do not publish a best single run. Do not claim comparisons with another product without authorized measurements under the same documented protocol.

## Pull requests

- Use a short Conventional Commit-style title that describes the change.
- Keep generated files deterministic and commit them with the source change.
- Separate unrelated changes into reviewable commits.
- Complete the pull request template and list every check actually run.
- Update documentation and `docs/STATE.md` when behavior, limitations, or measured results change.

Contributions are accepted under the repository's [MIT license](LICENSE).
