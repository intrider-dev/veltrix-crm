# Project state

Last updated: 2026-07-22

## Completed

- Installed and used the project-local `product-marketing` and `emil-design-eng` skills; the resulting product context is in `.agents/product-marketing.md`.
- Inspected the initially empty Windows workspace, initialized Git on `main`, and recorded architecture, performance, security, UX, and QA recommendations.
- Added centralized product and locale configuration, generated EN/RU catalogs, locale completeness checks, runtime language preference, and the workspace translation center.
- Implemented the Go modular monolith, PostgreSQL migrations/RLS roles, session authentication, RBAC, tenant guards, customers, sales, activities, reports, search, notifications/SSE, jobs/outbox, automation, integrations, attachments, deterministic seeds, and the Angular SPA feature routes.
- Recovered Docker Desktop and verified the pinned PostgreSQL `18.4-bookworm` Compose service is healthy.
- Applied all migrations to a clean real PostgreSQL database and passed the tagged integration suite, including negative tenant/RLS tests.
- Loaded and verified the deterministic `small` profile: 1,000 contacts, 250 companies, 500 deals, and 5,000 activities.
- Completed tenant-and-recipient-safe SSE live/replay filtering and the `search.sync`, `notification.dispatch`, and `automation.email.send` worker handlers.
- Added repeatable clean-dataset k6 orchestration: each measured run uses the isolated `veltrix-crm-benchmark` Compose project, retains raw summaries/resource samples, and produces a median report.
- Added portfolio documentation, ADRs, GitHub templates/workflows, backup/restore scripts, production multi-stage image, and optional Compose profiles.
- Rebranded runtime source/configuration to the centrally configured `VeltrixCRM` name and added the vector product mark without scattering brand literals through features.
- Added `/projects` and `/projects/:id` as a real PostgreSQL-backed slice with tasks, user/department responsible and watcher assignments, capabilities, attachments, and negative project RLS coverage.
- Added membership-scoped direct/group chat with bounded history, targeted SSE, attachments, reactions, pins, a responsive right-side dock, and explicit disabled call controls when no call provider is configured.
- Added deal list/Kanban/Gantt views with bounded server-driven loading and persisted view preference; AG Grid remains lazy and Community-only.
- Added calendar event audiences (`workspace`, `department`, `user`) with application predicates plus RLS, recipient-only SSE, restricted outbox suppression, protected child comments/reminders, EN/RU UI controls, and real PostgreSQL negative tests.
- Restricted pipeline and deal-stage configuration mutations to `settings.write`; ordinary `sales` users can no longer reconfigure the funnel through coarse record-update permission.
- Added disabled-by-default audio/video calls for authorized chat participants through an optional pinned LiveKit profile: compact room-scoped JWT grants, exact-origin CSP/Permissions Policy, recipient-only SSE, lazy browser client, responsive call UI, and PostgreSQL RLS/integration coverage. The base app + PostgreSQL profile is unchanged.
- Added workspace custom roles with immutable system-role envelopes, effective permissions loaded per tenant transaction, owner-only audited CRUD/assignment, optimistic version checks, RLS/composite-FK isolation, a lazy EN/RU role editor, and removal of the duplicated Angular role matrix.
- Added resource-specific lead/deal CRUD grants plus independent lead/deal funnel-management permissions, migrated existing role envelopes, permission-gated API/UI operations, and a PostgreSQL test proving a lead-only custom role cannot read deals.
- Added configurable lead stages and full deal pipeline/stage settings with real create/update/delete/reorder operations, ETags, idempotent creation, system-stage immutability, and lazy EN/RU settings routes.
- Added responsible and watcher assignments for leads, deals, and activities to individual users or departments, with primary responsibility, version checks, two-layer visibility, targeted SSE, and negative PostgreSQL isolation tests.
- Connected pipeline, deal-stage, and custom lead-stage names to the workspace translation center. APIs return source `name` plus locale-resolved `displayName`; reads are batch-resolved, source changes invalidate published translations to draft, and deletes remove translation resources transactionally.

## Current phase

Phase 3/4 — implement the personal corporate mailbox, close remaining end-user UI/E2E gaps, then run production-like hardening and measurements.

## Next actions

1. Implement the personal IMAP/SMTP mailbox slice with per-user encrypted credentials while keeping the base profile at app + PostgreSQL.
2. Add explicit per-stage transition rules and server-provided object capabilities where resource permissions and assignment visibility are not sufficiently granular.
3. Complete the optional disabled-by-default AI provider and document its exact verified surface.
4. Close remaining UI workflows, especially workspace bootstrap pipeline defaults, saved views, contact bulk/import/export, webhook delivery history, and chat presence/voice-message gaps.
5. Run the full lint/typecheck/unit/integration gates after mailbox generated code stabilizes.
6. Build and start the scratch production image against a clean Compose PostgreSQL volume; run the complete Playwright flow, including a two-browser call when the optional profile is enabled, and capture genuine screenshots.
7. Run Lighthouse, final bundle analysis, browser heap/DOM/interaction checks, three baseline k6 runs, Docker resource sampling, and security scans.
8. Update all reports with retained evidence, run independent reviews, fix high findings, and create logical Conventional Commits.

## Last successful verification commands

| Date | Command | Result |
| --- | --- | --- |
| 2026-07-21 | `docker compose config --quiet` and all optional profiles | Valid Compose configuration |
| 2026-07-21 | clean migrations `000001` through `000008` on PostgreSQL 18.4 | Passed |
| 2026-07-21 | `go test -tags=integration -count=1 ./...` from `apps/api` | Passed, including RLS/tenant negative tests |
| 2026-07-21 | `go run ./cmd/seed --profile small` from `apps/api` | Counts verified; completed in 2.684 s including `go run` startup |
| 2026-07-22 | `go test -count=1 ./...` and `go vet ./...` from `apps/api` | Passed after runtime worker/SSE changes |
| 2026-07-22 | `pnpm check:i18n`, `pnpm i18n:extract`, `pnpm check:forbidden` | Passed; 415 source keys, 2 locales |
| 2026-07-22 | `pnpm test:web` | 12 files and 25 tests passed |
| 2026-07-22 | `pnpm --filter @veltrix-crm/web lint` and `typecheck` | Passed before final OpenAPI regeneration |
| 2026-07-22 | Bash/PowerShell parser checks for benchmark scripts | Passed |
| 2026-07-22 | `pnpm lint` after projects, chat, rebrand, UI fixes, and calendar audience work | Passed; 783 source i18n keys and two complete locales |
| 2026-07-22 | `pnpm typecheck` after regenerated OpenAPI contracts | Passed |
| 2026-07-22 | `go test -tags=integration -count=1 ./apps/api/internal/activities` with PostgreSQL 18.4 | Passed; workspace/user/department visibility, exact-ID denial, targeted SSE, and no restricted outbox event verified |
| 2026-07-22 | targeted Go unit tests for activities, app, tenancy, collaboration, and events | Passed |
| 2026-07-22 | `go test ./...` from `apps/api` after calls implementation | Passed |
| 2026-07-22 | `go test -tags=integration -count=1 ./internal/calls` against PostgreSQL 18.4 | Passed; participant access, outsider denial, targeted SSE, and token omission verified |
| 2026-07-22 | `pnpm test:web` | 29 files and 78 tests passed |
| 2026-07-22 | `pnpm lint`, `pnpm typecheck`, and `docker compose --profile calls config --quiet` | Passed; 800 i18n keys, 2 complete locales, lazy LiveKit chunk generated |
| 2026-07-22 | `go test ./...` from `apps/api` after custom-role API/session integration | Passed |
| 2026-07-22 | `go test -tags=integration -run TestCustomRoleEffectivePermissionsOnPostgreSQL -count=1 ./internal/tenancy` | Passed against PostgreSQL 18.4; system-role parity, custom-role narrowing, non-owner denial, cross-workspace denial, owner protection, and assigned-role delete conflict verified |
| 2026-07-22 | `pnpm typecheck`, `pnpm test:web`, and web lint after custom-role UI | Passed; 29 files/78 tests and 828 complete EN/RU source keys |
| 2026-07-22 | targeted Go unit tests for localization, sales, app; then `go test ./...` | Passed after record assignments, resource permissions, configurable stages, and localized domain labels |
| 2026-07-22 | clean PostgreSQL 18.4 `TestDomainContentTranslationLifecycleOnPostgreSQL` | Passed; published RU resolution, source rename invalidation to draft, and source fallback verified |
| 2026-07-22 | `pnpm typecheck` and web lint | Passed after pipeline settings, lead/deal permission gates, and `displayName` integration; 893 complete EN/RU keys |

## Known limitations

- The final OpenAPI-generated state and optional AI implementation are still in progress; the full gate must be repeated after both land.
- The persisted automation rule model currently accepts the `scheduled` trigger but has no cadence/`next_run_at` producer. This is not claimed as complete and requires an explicit schedule model.
- Several advanced backend workflows do not yet have complete user-facing SPA/E2E coverage; they are listed under next actions rather than represented as complete UX.
- Custom roles now support independent lead/deal CRUD and funnel-management grants. Per-stage transition ACLs and generalized server-provided object capabilities remain in progress; current transitions are protected by resource update permission, assignment visibility, optimistic versions, and tenant RLS.
- Existing pipelines/stages created outside application services (notably deterministic seed SQL) still need a deterministic localization-resource backfill before they appear in the translation center; API display falls back to their source names safely.
- Chat currently provides text messages, files, reactions, pins, and optional real audio/video calls. Typing/presence, edit/delete, recorded voice messages, ringtone/device selection/screen sharing, and a two-browser media E2E run are not complete. Call controls stay disabled when no provider is configured.
- Personal corporate mailbox integration is not implemented. Existing SMTP is only for system notifications/password reset and is not represented as a user mailbox.
- Calendar audience isolation is implemented and tested, but a final independent review still recommends narrowing the database-level activity UPDATE policy so an assignee cannot use the runtime database role for arbitrary column updates; HTTP application queries already restrict full edits to creator or owner/admin.
- `go test -race` is unavailable in the current Windows Go environment because CGO is disabled. CI runs the race detector on Linux; that hosted result is not yet available.
- Production image/runtime, Lighthouse, baseline k6, browser heap/scrolling, security scanners, and portfolio screenshots have not yet been rerun after the latest changes.
- Hosted GitHub Actions have not run because no push is authorized.

## Actual measured metrics

| Metric | Value | Evidence/status |
| --- | --- | --- |
| Deterministic small seed | 1,000 contacts; 250 companies; 500 deals; 5,000 activities; 2.684 s including `go run` startup | Local PostgreSQL 18.4 run on 2026-07-21; setup evidence, not a serving-performance claim |
| Frontend initial JS + CSS | 86,727 B Brotli (84.7 KiB) | Dated pre-final bundle artifact; must be replaced after final build |
| Lazy AG Grid chunk | 158,063 B Brotli (154.4 KiB) | Dated pre-final bundle artifact; lazy-loaded |
| Largest ordinary lazy feature | 26,466 B Brotli (25.8 KiB) | Dated pre-final bundle artifact |
| External font requests | 0 | Pre-final bundle scan |
| Lighthouse / Web Vitals | Not measured | Final production-like run pending |
| Baseline k6 p95/p99/throughput/errors | Not measured | Three clean-dataset runs pending |
| Application/PostgreSQL RSS and CPU | Not measured | Time-series Docker sampling pending |
