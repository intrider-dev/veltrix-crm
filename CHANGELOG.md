# Changelog

VeltrixCRM is in active development. Until the first tagged release, changes are recorded under **Unreleased**. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); tagged releases will follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Product

- Added workspace membership, departments, configurable roles, resource permissions, stage permissions, invitations, session management, and TOTP MFA.
- Added contacts, companies, leads, deals, pipelines, activities, tasks, projects, calendars, saved views, custom fields, bulk actions, duplicate handling, and CSV import/export.
- Added list, Kanban, and timeline views for leads and deals with bounded server loading and optimistic conflict handling.
- Added team and record discussions with attachments, reactions, pins, recorded media, delivery states, and optional LiveKit calls.
- Added personal IMAP/SMTP mailboxes with encrypted credentials and durable outbound delivery jobs.
- Added dashboards, reports, PostgreSQL search, SSE notifications, audit history, automations, API keys, signed webhooks, and attachments.
- Added complete English and Russian interface catalogs, locale preferences, translation checks, and workspace content translations.

### Interface

- Reworked the application into the dark, high-density Veltrix Signal design system across authentication, navigation, dashboard, contacts, companies, leads, deals, activities, calendar, projects, settings, and messaging.
- Standardized buttons, fields, selects, segmented controls, toolbars, drawers, toasts, focus behavior, responsive layouts, and the local SVG icon set.
- Added accessible desktop, tablet, and mobile layouts with keyboard operation, focus containment, reduced motion, and WCAG checks.
- Added explicit build-update handling to prevent stale application shells after deployment.

### Platform

- Added an optional high-throughput profile with bounded Kafka domain-event publication and RabbitMQ command routing, strict pointer envelopes, publisher confirmations, a local dual-broker smoke scenario, and an explicit comparative benchmark gate. The PostgreSQL-only base profile remains unchanged.
- Added the Angular SPA, Go modular monolith, OpenAPI contract, PostgreSQL migrations, deterministic seeds, and a two-container production profile.
- Embedded precompressed SPA assets into a non-root Go runtime image without Node.js.
- Added PostgreSQL-backed jobs, transactional outbox processing, retry and dead-letter states, and tenant-scoped SSE replay.
- Added reproducible browser and load scenarios, bundle reports, backup/restore scripts, optional service profiles, and repository workflows.

### Security

- Added application authorization and forced PostgreSQL RLS with transaction-local workspace context.
- Added negative tenant tests for reads, writes, search, export, activity, stages, chat, calls, attachments, and jobs.
- Added Argon2id password hashing, hashed session and API secrets, CSRF protection, request limits, secure headers, audit records, and scoped webhook signatures.
- Isolated privileged migration credentials from the application container and kept application encryption keys out of PostgreSQL.

### Known limitations

- Hosted workflow, CodeQL, container scan, SBOM, and Linux race results must be confirmed from repository runs; local configuration alone is not reported as a pass.
- Server-backed delivered/read receipts and a searchable assignment combobox remain planned.
- The 100-VU stretch profile and a two-browser call scenario have not been measured.

For current verification evidence, see [docs/STATE.md](docs/STATE.md) and [docs/PERFORMANCE.md](docs/PERFORMANCE.md).
