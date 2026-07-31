# Changelog

All notable changes will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project intends to use [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once releases begin.

## [Unreleased]

### Added

- Personal per-user IMAP/SMTP mailboxes with encrypted write-only credentials, forced user-scoped RLS, SSRF-safe bounded transports, plain-text rendering, lazy Angular mail UI, and transport/service/integration tests.
- Per-role lead/deal stage visibility and transition overlays with atomic audited configuration, RLS, database-level read filtering, localized Material editors, and negative PostgreSQL tests.
- A documented single Material component layer for consistent buttons, fields, selects, focus, density, errors, and bounded toast feedback.
- Initial modular-monolith, Angular SPA, PostgreSQL schema, OpenAPI contract, deterministic seed, localization, Docker, benchmark, and documentation work in progress.
- Bilingual portfolio README, architecture and case study; threat model, benchmark and competitor protocols, dependency inventory, demo/release guidance, and evidence-only screenshot workflow.
- First-class English/Russian catalogs, locale-preference hierarchy, translator checks, and a tenant-owned content translation workflow with coverage and draft/published states.
- GitHub Actions definitions for lint/typecheck, backend/frontend/integration/E2E tests, production builds, dependency auditing, CodeQL, container scanning/SBOM, PR benchmark smoke, and manually dispatched full benchmarks.
- Contribution, conduct, security, issue, pull-request, and dependency-update policies.

### Changed

- Replaced the previous visual layer with the `Veltrix Signal` design system: warm neutral canvas, evergreen/lime hierarchy, consistent Material controls, compact list workspaces, redesigned dashboard/reports, responsive record details, accessible modal drawers, and a polished light/dark messenger.
- Added list/Kanban/Gantt view switching to lead and deal workspaces, clarified accepted-message status, made recorded media explicitly loadable/playable, and hardened mobile navigation/chat/drawer focus behavior after independent review.
- Rebranded the centrally configured product and repository paths to VeltrixCRM, added the vector product mark, and kept feature code free of scattered brand literals.
- Added deal list/Kanban/Gantt views, projects/tasks, resource and stage ACLs, departments, assignments/watchers, calendar audiences, chat, optional LiveKit calls, and personal corporate mail as complete database-backed vertical slices.
- Unified Material controls, corrected chat/contact/company layout regressions, and made the scrollable Kanban keyboard-focusable.
- Moved the password-recovery action below the password field, preserved logical keyboard order, and prevented the product name from breaking inside the word at compact desktop widths.
- Added explicit PWA version handling: ready builds reload automatically on authentication routes and use a persistent localized reload toast inside the CRM shell, avoiding silent stale app-shell sessions after deployment.
- Fixed the zoneless direct-chat member selector so the Create action reacts immediately, localized its placeholder, and expanded production-browser acceptance to calendar creation, all deal views, projects, and bidirectional owner/invited-user messaging.
- Split stored dashboard summaries from stage-filtered fallback aggregation and bounded the authorized global-search plan after reproducible performance regressions.

### Security

- Added application guards plus negative PostgreSQL tests for export/search/activity/stage/chat/call isolation, checksummed migrations, and fenced PostgreSQL job leasing/retry behavior.
- Moved migration/demo credentials into a bootstrap wrapper derived from the pinned PostgreSQL image. The base profile now retains exactly PostgreSQL plus the non-root scratch app; privileged database credentials never enter the app container, the application encryption key never enters PostgreSQL, bootstrap variables are scrubbed before the final database process, and Node.js is absent at runtime.

- CI policy definitions include forced generated-state checks, forbidden dependency checks, `govulncheck`, production `pnpm audit`, license review, CodeQL, Trivy, and an SPDX SBOM.

> Workflow definitions have been added, but their successful execution on GitHub-hosted runners is not claimed here. Consult workflow runs and `docs/STATE.md` for verified status.
