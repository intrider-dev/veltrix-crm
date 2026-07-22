# Roadmap

The roadmap describes intended outcomes, not promises or completed functionality. Verified status belongs in `docs/STATE.md`; measured performance belongs in `docs/PERFORMANCE.md`.

## 0.1 — complete production vertical slice

- Login/session security, workspace selection, RBAC, application guards, and forced PostgreSQL RLS.
- Dashboard, contacts, companies, deals/pipeline, activities, audit trail, real migrations, deterministic demo/small seeds, and an end-to-end user path.
- English/Russian UI and system messages, user/workspace locale settings, typed catalogs, and translator checks.
- Two-container production-like Compose path with embedded, precompressed SPA assets and no Node.js in runtime.
- Unit, PostgreSQL integration, Playwright smoke, accessibility, bundle, and security gates.

## 0.2 — core Sales CRM workflows

- Complete contacts/companies/leads/deals/pipelines with custom fields, saved views, bulk actions, duplicate merge, CSV import/export, and cursor-driven lists.
- Tasks, meetings, calls, notes, comments, reminders, calendar/ICS, notifications over SSE, global PostgreSQL search, dashboards, reports, and forecast views.
- Translation workflow documentation and additional locale proof-of-process without machine-translating user content.

## 0.3 — bounded advanced capabilities

- Idempotent automation engine, PostgreSQL jobs/outbox, retries/dead letters, execution previews, and recursion/rate safeguards.
- Scoped hash-only API keys, signed/replay-resistant webhooks, streaming attachments, optional SMTP/S3 adapters, invitations, password reset, and TOTP MFA.
- Optional, disabled-by-default AI provider interface with explicit external-PII consent.
- PWA app shell and explicitly bounded offline drafts.

## 0.4 — hardening and reproducible evidence

- Independent tenant/security/maintainability reviews and fixes for critical/high findings.
- SQL/bundle/CPU/RSS/browser-heap profiling, memory-leak scenarios, Lighthouse/accessibility checks, baseline and stretch k6 runs, and documented missed budgets.
- Backup/restore exercises, container hardening, SBOM/vulnerability evidence, and failure/recovery tests.

## 1.0 — documented stable release candidate

- Clean-clone verification, production-like migration/seed/user flow, screenshots from the running product, bilingual README/case study, threat model, benchmark methodology/results, and release notes.
- No undocumented core TODOs, fake metrics, fabricated screenshots, or unverified competitor claims.
- A support policy based on actual maintainer capacity and release history.
