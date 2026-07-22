# VelocityCRM Lab — master requirements

Captured: 2026-07-21  
Repository working name: `velocity-crm-lab`  
Product working name: `VelocityCRM Lab`

This document preserves the implementation brief supplied to the coding agent. It is the project scope and truthfulness contract. Later decisions may clarify it in ADRs but must not silently weaken it.

> Historical naming note: on 2026-07-22 the project owner renamed the product to
> **VeltrixCRM** and the repository/runtime prefix to `veltrix`. The original working
> names above are retained only because this file records the source brief verbatim.

## Subsequent requirement — product expansion and rebrand

Added by the project owner on 2026-07-22:

- Deals must switch between list, Kanban, and Gantt views.
- Buttons, selects, and inputs need one polished visual system; Angular Material/CDK
  remains the single UI kit.
- Fix the duplicate search clear icon, icon/text alignment, native select arrows,
  lead form padding, company translations, and calendar event creation.
- Show live notifications as toasts while preserving the notification inbox.
- Add departments, flexible roles and scoped permissions for leads, deals, stages,
  projects, and tasks.
- Add configurable lead/deal stages, project task groups, responsible users or
  departments, watchers, deadlines, attachments, and entity conversations.
- Add a modern right-side user chat with messages, reactions, pins, attachments,
  voice messages, and a defensible voice/video call architecture.
- Add personal, department, and workspace calendar events.
- Add user-configurable corporate mailbox adapters and a full mailbox section.
- Rename the product to VeltrixCRM, use the `veltrix` prefix in project paths, and
  add a production-ready vector logo throughout the product.

## Subsequent requirement — verbatim

Added by the project owner on 2026-07-21:

> Нужно чтобы проект еще сразу был мультиязычный, с идеальной и очень удобной системой доработки мультиязычности, перевода контента и сообщений внутри, сменой языка в настройках
>
> Продолжай

## Mission

Design, implement, test, benchmark, and document a production-grade, open-source, multi-tenant Sales CRM SPA. It must be a real application with PostgreSQL, authentication, tenant isolation, CRUD, business logic, reproducible performance scenarios, and GitHub portfolio documentation—not a static mock or disconnected UI demo.

Centralize all brand references so the product can be renamed without mass source edits.

Engineering priorities, in order:

1. Extremely responsive UI.
2. Low memory, CPU, and network use.
3. Minimal infrastructure components.
4. Useful operation for small/medium teams on modest servers.
5. Performance on large CRM datasets.
6. Honest, reproducible measurements.
7. Public-portfolio-quality architecture and code.

Never claim superiority over a named CRM without a reproducible measurement. Never invent metrics, screenshots, integrations, tests, or results.

## 1. Working mode

- Inspect the existing directory and tools first; preserve user files.
- Initialize Git on `main` when absent.
- Create and maintain `AGENTS.md`, `docs/EXECUTION_PLAN.md`, `docs/STATE.md`, this file, and `docs/adr/`.
- `docs/STATE.md` must record completed/current/next work, last successful verification commands, known limitations, and actual metrics.
- Plan, then implement continuously through the Definition of Done. Do not stop at analysis, an architecture document, or scaffolding.
- Make safe library, naming, color, and directory decisions independently. Ask only for a missing secret/account, a genuinely destructive operation, or a value with no safe default.
- Use read-only architecture, performance, security, UX/accessibility, and QA subagent reviews early; later use independent security, performance, test-gap, and maintainability reviews. The primary agent integrates decisions.
- After every major stage: format, lint, typecheck, test, fix failures, update state, inspect diff, and create a small Conventional Commit if Git identity already exists. Never change global identity; record proposed messages in `docs/COMMITS.md` otherwise.
- Do not push, force-push, or change remotes.
- Use stable, pinned dependencies and lockfiles. Avoid experimental/deprecated APIs unless a critical reason is documented. Explain every production dependency and license in `docs/DEPENDENCIES.md`.
- Prefer small, tested standard-library implementations over unnecessary dependencies.

## 2. Fixed technology stack

### Frontend

- Angular 22 SPA, standalone components, strict TypeScript, no SSR.
- Zoneless Angular with no `zone.js`.
- Signals for local/feature state; Angular Signal Forms; Angular Router.
- Angular Material 22 as the only general UI kit; Angular CDK and Angular Aria for behavior, DnD, overlays, accessibility, and virtualization.
- AG Grid Community only for complex list screens. Never install/import Enterprise. Register used Community modules only and load the grid solely inside lazy feature routes.
- Prefer small custom SVG charts; compare bundle size before adding a lazy community chart library and record the decision in an ADR.
- Angular's standard test runner and Playwright for E2E/browser performance.
- PWA app shell and offline-friendly drafts, not full offline-first replication.

Frontend prohibitions: no second universal UI kit, Tailwind, icon font, external fonts, whole icon sets, Moment.js, full Lodash, unjustified global Redux-like store, auth tokens in localStorage, whole-dataset browser loads, or unjustified `any`.

### Backend

- Latest stable Go compatible with the environment; modular monolith.
- `net/http`, a small router (prefer `chi`), `pgx`, and `sqlc` or equivalent generated type-safe SQL; no heavy ORM.
- OpenAPI 3.1 contract, REST under `/api/v1`, RFC Problem Details errors.
- SSE for notifications/progress; Go structured logger; health checks.
- `pprof` only in development/benchmark. Metrics disabled or near-zero-cost in idle production.

### Storage and deployment

- PostgreSQL 18, or current stable available major, with pinned image; `pg_trgm` and built-in full-text search.
- PostgreSQL also stores transactional outbox, jobs/retries, automation execution, and global search documents.
- No Redis, Elasticsearch/OpenSearch, Kafka, RabbitMQ, separate search service, or workflow service in the base profile.
- One production Go binary serves API, SSE, workers, Angular assets, and SPA fallback. Embed fingerprinted assets and prebuilt gzip/Brotli variants; negotiate `Accept-Encoding`.
- Runtime image contains no Node.js; multi-stage, pinned, minimal, non-root.
- Base production Compose: app + PostgreSQL only. The worker can run separately later, but is in-process by default.
- Optional Compose profiles: `mail` (Mailpit), `object-storage` (MinIO), `ai-local` (Ollama-compatible config), and `benchmark` (limits/dataset). None is required for core CRM startup.

## 3. Architecture principles

Use a modular monolith with clear identity, tenancy, contacts, companies, sales, activities, automation, notifications, reporting, search, files, integrations, and platform boundaries. Organize `apps/web`, `apps/api`, `packages/contracts`, `packages/design-tokens`, `benchmarks`, `docs`, `infra`, `scripts`, and `.github` as useful, without mechanically creating empty directories.

- A module may not reach into another module's internal tables without an explicit interface.
- Avoid speculative abstractions, artificial micro-services/classes, giant services, and giant components.
- Frontend data flow is one-way with feature-scoped signal stores. Use RxJS for cancelable streams, SSE, complex async composition, and Angular integration—not every value.
- API types originate in OpenAPI; do not hand-maintain incompatible Go/TS DTOs.
- Duplicate-prone mutations accept idempotency keys. Concurrent edits use `version` and/or ETag/`If-Match`.
- UTC timestamps; money as integer minor units plus ISO currency.
- Use UUIDv7 or another index-friendly unique ID.
- Parameterized SQL, no N+1, cursor pagination for large lists; offset only for small reference data.

## 4. Multi-tenancy and data model

Core concepts include User, Session, MFA configuration, Workspace, membership, Team, Role, Permission, Invitation, Contact, Company, Lead, Pipeline/Stage, Deal/history, Activity/Task/Meeting/Call/Note, Comment/mention, Reminder, Tag, Custom Field definitions/values, Saved View, Notification, Automation Rule/Execution, Background Job, Audit Event, Attachment, API Key, Webhook Subscription/Delivery, Outbox Event, and Search Document.

Every tenant-owned table has `workspace_id`. Enforce isolation twice:

1. Mandatory application guards and object authorization.
2. PostgreSQL RLS defense in depth.

Use separate database roles. Inside each transaction set tenant context with `SET LOCAL`; never leave it on pooled connections. System workers use a separate controlled path. Add negative real-PostgreSQL tests proving workspace A cannot read, update, delete, export, or search workspace B. Compound indexes begin with `workspace_id`.

Custom field types: text, number, money, date, boolean, single select, multi select, user reference. JSONB is allowed with type validation, migration strategy, size caps, type-safe API, and only required GIN indexes.

Global search uses `search_documents` with tenant, entity type/id, title, subtitle, normalized text, `tsvector`, trigram-compatible values, rank, and safe snippets. Search contacts, companies, leads, deals, and notes. Update atomically or through reliable outbox work.

## 5. Functional Sales CRM scope

### Identity/workspaces

Development/demo registration, login/logout, secure session cookies, password reset through local SMTP profile, password change, TOTP MFA, recovery codes, workspace create/switch, email invitations, members, team membership, disable member, terminate all sessions, and owner/admin/manager/sales/viewer permission matrix.

### Contacts/companies

CRUD, soft delete/trash/restore, owner/team/tags/custom fields, addresses/phones/emails, source/status, last contacted/next activity, relationship, search/sort/filter, saved views/columns, bulk select/assign/tags/delete, CSV preview/mapping/import/progress/error report/export of the filtered view, email/phone normalization, duplicates and merge, immutable audit trail.

### Leads/deals/pipelines

Multiple configurable pipelines/stages/probabilities, lead conversion, amount/currency/expected close/owner/participants/simple line items, won/lost/reason, stage history, forecast category, lazy-per-stage Kanban DnD with optimistic rollback/version conflict, filters, saved/list views, analytics and forecast.

### Activity/collaboration

Tasks, calls, meetings, notes, comments, mentions, reminders, assignment/due date/priority/minimal recurrence, entity timelines, global feed, internal day/week/month calendar, ICS export, attachments, in-app notifications, and email notifications through local SMTP adapter.

### Dashboard/reports

KPI cards, open pipeline, weighted forecast, won/lost, conversion, deals by stage/owner, overdue tasks, activities by period, lead sources, recent activity, period selector, workspace timezone, saved dashboard preferences. Avoid heavy aggregation per render; use indexes, bounded cache, or worker-maintained summaries where justified.

### Automation

Real engine with triggers (create/update, stage change, won/lost, overdue, scheduled), conditions (equal/not equal/contains/numeric/date/tag/owner/team with AND/OR), and actions (task, assign, tags, notification, email, webhook, allowed field update). Require idempotency, execution log, exponential retry/max attempts/dead-letter, recursion protection, safe preview, enable/disable, rate limit, and tenant isolation.

### API/webhooks

OpenAPI 3.1 and development docs, scoped API keys stored only as hashes with last-used/revoke, HMAC webhooks with replay defense/delivery log/retry/manual retry/secret rotation, request/response limits, cursor pagination, validation, and consistent problems.

### Attachments

Storage interface with local filesystem default and optional S3-compatible adapter; generated safe names plus original display name, MIME and size validation, streaming, authorization, antivirus hook, and traversal prevention.

### Optional AI

Provider interface for Ollama-compatible local and documented OpenAI-compatible adapters. Optional timeline summary, follow-up draft, next action, and duplicate suggestions. Disabled by default; hidden without provider; explicit consent before external PII transfer; timeout/cancel/rate limit/audit; no external data without explicit configuration.

## 6. UI, UX, accessibility, and localization

Build a professional compact SaaS interface with Material 3 tokens, light/dark themes, comfortable/compact density, collapsible left navigation, top bar, workspace switcher, global search, command palette, useful breadcrumbs, master/detail, quick drawer and full edit pages, skeleton/empty/inline-error/retry/optimistic states, short-lived toasts only, and persistent panels for important errors.

Routes: `/login`, `/dashboard`, `/contacts`, `/contacts/:id`, `/companies`, `/companies/:id`, `/leads`, `/deals`, `/deals/:id`, `/activities`, `/calendar`, `/automations`, `/reports`, `/notifications`, `/settings`, `/settings/members`, `/settings/custom-fields`, `/settings/api`, `/settings/webhooks`, `/settings/audit`.

Support desktop/tablet/reasonable mobile, keyboard/focus management, discoverable shortcuts, reduced motion, high contrast, screen readers, WCAG 2.2 AA, and native `Intl` dates/numbers with workspace timezone.

Multilingual requirements (added immediately after the master brief):

- RU and EN from the first working slice, including UI, validation, errors, notifications, emails, and system content.
- An exceptionally convenient extension workflow for translators and developers, with centralized catalogs, typed/stable keys, documented addition of a language, completeness and placeholder checks, and no scattered literal strings.
- User language setting in Settings; user preference overrides workspace default. Workspace admins choose the workspace language/default locale.
- User-entered CRM text is not silently machine-translated. System templates store keys/typed parameters and render in the intended recipient locale.
- API logic relies on stable codes rather than translated prose.

No remote fonts or heavy decorative core imagery.

AG Grid appears only on lazy list routes, uses server cursor/infinite data source, never keeps 100k rows in browser memory, supports keyboard navigation/selection/sort/filter/resize/reorder/saved column state/bulk actions/accessible labels/limited rendered rows, and never Enterprise APIs.

Kanban loads bounded cards per stage, uses CDK DnD, persists order, optimistically moves with rollback, and reports version conflict. CSV preview/heavy transformations run in a Web Worker or backend.

## 7. Frontend performance

Use zoneless Angular, OnPush, signals/computed, effects only for side effects, tracked `@for`, `@defer`, route/component lazy loading, selective imports, cancellation, `takeUntilDestroyed`, ETag/304, request dedupe, bounded feature cache, virtualization/pagination, ~150 ms global search debounce, selective small-route prefetch, and immutable fingerprinted asset caching.

IndexedDB only stores form drafts, recent workspace metadata, small offline app state, and explicitly supported draft operations—not the database.

Add build budgets and bundle reporting:

- Initial JS+CSS target ≤350 KiB Brotli; warn >400; fail >450.
- Normal lazy feature target ≤200 KiB Brotli; document larger lazy AG Grid/report chunks.
- No external font requests.
- Lighthouse desktop performance ≥95, mobile ≥90, accessibility ≥95.
- LCP ≤2.0 s in fixed local mobile emulation; CLS ≤0.05; INP/interaction latency ≤150 ms for key actions.
- Prefer active DOM ≤1500; large table scroll ≥55 FPS on benchmark machine; browser heap preferably ≤200 MB; retained heap growth after 20 list-details-list cycles ≤15%.

For a missed budget, execute at least two distinct optimization iterations, rerun the same test, keep functionality unless justified, and record actual results/reason in `docs/PERFORMANCE.md`.

## 8. Backend performance/resources

Base app target: idle RSS ≤96 MB; startup to readiness ≤1 s; bounded goroutines/caches/queues/request bodies/concurrency; streaming export/import/download; graceful shutdown; bounded DB pool and memory-aware limits. App+PostgreSQL should strive to fit 512 MB. Measure actual Docker resource use.

Deterministic synthetic seeds:

- `small`: 1,000 contacts, 250 companies, 500 deals, 5,000 activities.
- `benchmark`: 100,000 contacts, 25,000 companies, 50,000 deals, 500,000 activities.

Baseline k6: app 0.5 CPU/128 MB, PostgreSQL 0.5 CPU/384 MB, 50 VUs, realistic list/read/search/write mix, multi-minute duration after warmup. Goals: error <0.5%, p95 reads <150 ms, writes <250 ms, global search <250 ms, no OOM/pool starvation/unexplained query degradation; always report throughput and p99. Stretch: 100 VUs, report degradation/bottleneck without treating it as baseline failure.

Create `benchmarks/k6`, `benchmarks/browser`, `benchmarks/results/.gitkeep`, cross-platform benchmark scripts, `BENCHMARK_METHODOLOGY.md`, `PERFORMANCE.md`, and `WHY_FAST.md`.

Every report includes date, commit, OS/CPU/RAM, Docker/browser, dataset, limits, cold/warm, runs, median/p95/p99/throughput/errors, container RSS/CPU, bytes, and bundles. Use several runs and median—not the best result.

## 9. Security

Implement and test Argon2id, cryptographic session tokens stored only as hashes, rotation/expiration/logout/logout-all, HttpOnly/Secure-in-production/SameSite cookies, CSRF, login/reset/API rate limits, generic auth errors, backoff/temporary lock, TOTP, hashed recovery codes, CSP, nosniff, Referrer/Permissions policies, correctly gated HSTS, encoding/no unsafe HTML, validation/parameterized SQL, tenant isolation/RBAC/object authorization, append-oriented audit, PII/secret log redaction, request IDs, safe upload, HMAC webhooks, API scopes, safe `.env.example`, and same-origin/CORS-disabled default.

Audit includes actor, tenant, action, entity, UTC time, request ID, safe change summary, and permitted IP/user-agent—never passwords/tokens/secrets.

Add govulncheck, frontend audit, static analysis, container scan, CodeQL, review checklist, `SECURITY.md`, and threat model. Never bypass a security test for CI.

## 10. Jobs, outbox, and real-time

PostgreSQL job queue uses `FOR UPDATE SKIP LOCKED`, lease/attempts/availability/lock/worker/timeouts/exponential backoff/dead-letter/idempotency/graceful shutdown/configurable concurrency.

Domain mutations and outbox rows share a transaction. Dispatcher creates notifications, automation jobs, webhook deliveries, search jobs, and SSE events.

SSE endpoint is tenant-scoped and authorized with heartbeat, reconnect, `Last-Event-ID`, bounded replay, no cross-tenant leak, cleanup, and backpressure. Do not poll constantly when SSE exists.

## 11. Testing

Backend: unit/repository/real-PostgreSQL integration/migration/RLS/tenant/API-contract/auth/authz-matrix/automation-idempotency/webhook/job-retry/race-sensitive tests and `go test -race` where possible.

Frontend: component/store/form/error/permission/keyboard/localization/theme tests.

Playwright E2E must cover login, workspace creation/invite, company/contact/link, deal/Kanban, task completion, global search, saved view, CSV import/export, automation, notification, audit, second-tenant denial, dark theme, and axe scans. Create real stable-page visual smoke screenshots at 1440×900, 1024×768, and 390×844; avoid blanket pixel snapshots.

Canonical root commands: `pnpm bootstrap`, `dev`, `build`, `lint`, `typecheck`, `test`, `test:e2e`, `test:integration`, `benchmark`, `seed:small`, `seed:benchmark`, `check`. A Makefile/scripts may supplement; `docker compose up --build` remains primary production-like launch.

## 12. Development and production experience

From clean clone: copy `.env.example` to `.env`, run Compose, open app, login with documented local-only `admin@demo.local` / `Demo123!`. Never seed it in production mode.

Provide editor/git/docker ignores/configs, env example, Compose/Dockerfile, Linux/macOS and PowerShell scripts, deterministic seeds, backup/restore, graceful migration, liveness/readiness, changelog/SemVer.

Runtime is non-root, read-only where practical with explicit writable volume/tmp, healthcheck, ideally no shell, pinned minimal base/package set, SBOM and vulnerability scan.

## 13. GitHub portfolio

Create bilingual README and architecture/case study; MIT license, contributing, code of conduct, security, changelog, roadmap, agents guide, why-fast, performance/methodology/competitor protocol/threat model/demo script/AI build log/GitHub setup, ADRs, issue/PR templates, Dependabot, and Actions.

README starts with value proposition and includes description, architecture idea, real features/screenshots/measured performance/resource profile, quick start/demo credentials, Mermaid, repo structure, test commands, security, roadmap, AI disclosure, and links. Never use unsupported “fastest”, “zero-latency”, “enterprise-ready”, “infinitely scalable”, or competitor-beating claims. `Not measured` is preferable to fiction.

Case study covers problem/constraints/hypotheses/architecture/modular-monolith rationale/no Redis-Kafka-Elasticsearch/zoneless Angular/bundle control/tenant isolation/outbox/jobs/method/results/failed decisions/tradeoffs/Codex role/verified decisions/limitations/roadmap.

AI build log contains verifiable requirements, stages, changes, checks, fixed issues, and AI-assisted files—not hidden reasoning or invented history.

Use Playwright against the real app for dashboard, contacts grid, pipeline, details timeline, reports, and dark-theme screenshots. `GITHUB_SETUP.md` includes description/topics/release title/body/publication order/social images/commit suggestions.

## 14. Competitor benchmark protocol

Document manual comparison of cold/warm load, contact list/search/details/create, deal create/move, filter, dashboard, 10-minute memory, bytes/requests, idle server memory, and load CPU. Require identical computer/browser profile/network/approximate data, repeated runs, date/version, DevTools evidence, allowed HAR/trace, and Terms of Service compliance. Never automate another product without permission and never publish unmeasured values; use `Not measured`.

## 15. CI/CD

GitHub Actions: lint/typecheck, backend tests, frontend tests, integration tests, E2E smoke, production build, dependency security, CodeQL, container scan, benchmark smoke, and manual full benchmark. PRs run stable smoke only; full artifacts are manual.

Use caches, no secrets, PostgreSQL service/migrations/seed, Playwright failure traces, bundle report, production container, runtime-no-Node check, no-Enterprise check, license policy, and clean generated-code check.

## 16. Delivery phases

0. Discovery/planning, tools, plan/ADR/model/OpenAPI/benchmark; immediately continue.
1. Green vertical production slice: login → dashboard → contacts/create/details → company → deal/pipeline → activity/audit across migration/API/UI/validation/auth/tests/Docker/seed.
2. Core CRM: complete records/pipelines/activities/views/fields/search/CSV/notifications/reports.
3. Advanced: automation/keys/webhooks/files/invites/MFA/optional AI/PWA/calendar/email.
4. Hardening: tenant/security/performance/SQL/bundle/memory/load/accessibility/errors/backup/container.
5. Portfolio: bilingual docs/case study/screenshots/diagrams/results/templates/CI/release/final review.

Prefer a complete vertical slice over dozens of unfinished screens.

## 17. Definition of Done

Done means a clean clone starts; base is app+PostgreSQL and runtime has no Node; real login, multi-tenancy, RBAC, contacts/companies/leads/deals/pipelines, timeline/tasks/calendar/search/views/bulk/custom fields/CSV/SSE/automation/API keys/webhooks/audit/dashboard/reports all work; backend/frontend/integration/E2E tests pass; core console is clean; no undocumented core TODO or fake metrics; real bundle/benchmark/screenshots exist; bilingual README/case study/threat model/competitor protocol exist; workflows pass or deviations are documented; state is factual; diff reviewed; final answer claims only verified work.

## 18. Final verification

Before final response: clean build, unit/integration/E2E, production image, production-like Compose, clean DB migrations, small seed, main user flow, browser console, screenshots, Lighthouse, bundles, baseline k6, Docker metrics, security scans, tenant tests, actual docs, independent reviews, critical/high fixes, and reruns.

For an unavailable tool record the failed check, exact reason, developer command, and unverified scope. Never fabricate a substitute result.

## 19. Final response

Respond in Russian with: implemented scope, launch, demo credentials, commands, production deployment, actual metrics, met/missed budgets, optimizations, test/security results, README/case/performance/screenshot paths, commits/proposals, Git status, and honest remaining limits. Continue while implementation is possible; stop only at an objective blocker.

## 20. Subsequent product requirements

The user subsequently extended the scope with these requirements. They are part of the product brief and must be implemented and verified with the same evidence standards as the original requirements:

- Make the application multilingual from the start, with a maintainable translation workflow for interface text, validation, server messages, notification content, and workspace-authored translatable content. Provide a language switcher in settings.
- Support deal presentation as list, Kanban, and Gantt views.
- Use one polished component language for buttons, selects, inputs, segmented controls, and field groups. Angular Material remains the only universal UI kit; a small shared component/token layer may refine it.
- Fix the reported UI defects: duplicate clear icon in search, misaligned plus/text inside buttons, native select arrows touching the edge, insufficient padding in the new-lead field container, cramped contacts/companies toolbars, malformed square/circular segmented controls, and the malformed chat creation layout.
- Deliver transient notifications as accessible toasts while retaining persistent panels for errors that need user action.
- Add built-in team chat and calls, including a right-side chat dock, instant messages, attachments, voice messages, video/voice calls, emoji reactions, and pinned messages. Where an external maintained real-time media component is used, it must remain optional and must not add required infrastructure to the base deployment.
- Add custom roles, departments/teams, administrative rights, resource permissions, and stage-level permissions for both deal and lead pipelines.
- Add projects as permission-controlled task groups. Tasks, leads, and deals support responsible users/departments and watchers; their collaboration threads and attachments are visible only to authorized users.
- Calendar events must be creatable and support workspace-wide, department, and user audiences.
- Add a personal corporate mailbox area with per-user mail configuration, inbox/sending, and the expected core mail workflow while keeping secrets encrypted and tenant-isolated.
- Rebrand the product and repository paths from Velocity/VelocityCRM Lab to the centrally configured `VeltrixCRM` / `veltrix-crm` identity.
- Create and place a distinctive vector logo whose colors and brand text are controlled through central design/product configuration.

Latest visual feedback specifically identifies the chat creation panel and the contacts/companies filter bars as cramped or geometrically inconsistent. These layouts require responsive regression coverage at desktop, tablet, and mobile viewports.
