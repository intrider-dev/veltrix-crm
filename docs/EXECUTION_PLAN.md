# Execution plan

Last updated: 2026-07-21

## Phase 0 — discovery and decisions

- Inspect repository, OS, toolchain, Docker, and Git identity.
- Initialize Git on `main`; capture the master requirements, state, dependency policy, architecture, i18n model, benchmark method, and ADRs.
- Resolve stable dependency versions and generate pinned lockfiles.
- Define module boundaries, PostgreSQL schema/RLS, OpenAPI contract, brand config, locale catalogs, and acceptance gates.

Exit: architecture is actionable, core commands are defined, no production code relies on an undecided security boundary.

## Phase 1 — production vertical slice

Implement and verify this path against real PostgreSQL:

`login → dashboard → contacts → contact details → company → deal → pipeline → activity → audit event`

Includes migrations, deterministic demo seed, Argon2id sessions, CSRF, tenant guards/RLS, RBAC, API validation/errors, Angular routes/forms/stores, RU/EN locale switching, Docker build, unit/integration/E2E tests.

## Phase 2 — core Sales CRM

Finish contacts/companies/leads/deals/pipelines, tasks, timeline, saved views, custom fields, bulk actions, CSV import/export, global PostgreSQL search, notifications/SSE, dashboards/reports, and calendar basics.

## Phase 3 — advanced capabilities

Implement automation/outbox/jobs, invitations/members, MFA/recovery, API keys, signed webhooks, attachments, email adapter, PWA drafts, and disabled-by-default AI providers.

## Phase 4 — hardening and measurement

Run tenant/RLS negative tests, authorization matrix, race tests, dependency/static/container scans, SQL and bundle profiling, accessibility checks, memory/resource measurements, baseline/stress k6, backup/restore, and clean-database deployment.

If a budget fails, perform at least two distinct optimization iterations and record every measured result without weakening the scenario.

## Phase 5 — portfolio release

Generate only real Playwright screenshots from the running product, complete bilingual README/architecture/case study, benchmark and competitor protocols, GitHub templates/workflows, release guidance, AI build log, final independent reviews, and a truthful release readiness report.

## Review cadence

After each phase:

1. Format, lint, typecheck, unit/integration/E2E checks proportional to the change.
2. Update `docs/STATE.md` and measured reports.
3. Inspect diff and generated-code cleanliness.
4. Commit a small Conventional Commit when repository identity exists; otherwise append proposals to `docs/COMMITS.md`.
