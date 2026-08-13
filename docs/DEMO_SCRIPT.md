# Demo script

This is a reproducible portfolio walkthrough against the real application, not a mock. Allow 15–20 minutes. Use synthetic data only.

## Preflight

```bash
cp .env.example .env
docker compose up --build
```

PowerShell uses `Copy-Item .env.example .env`. Wait for both health checks, then open <http://127.0.0.1:8080>.

Development-only credentials:

- `admin@demo.local`
- `Demo123!`

Before presenting, confirm [`STATE.md`](STATE.md) records a successful clean migration, small seed, and browser smoke for the current commit. Do not present an unchecked route as complete.

## Story

### 1. Explain the operational premise

Show `docker compose ps` and the base `compose.yaml`: only the app and PostgreSQL are required. Explain that Node.js is build-only; the Go binary serves API, SSE, workers, and embedded precompressed SPA. Do not quote RSS or startup values unless a current artifact exists in [`PERFORMANCE.md`](PERFORMANCE.md).

### 2. Login and workspace

Sign in with the demo user. Point out the HttpOnly cookie model (without exposing cookie values), workspace switcher, compact navigation, command palette, theme control, and locale control.

Expected: dashboard appears, the selected workspace is visible, and the browser console has no error.

### 3. Multilingual workflow

Open Settings and switch English → Russian → English. The loaded UI should update immediately and the preference should survive navigation. Show workspace language/default settings, then open the Translation Center:

- filter content by namespace/status;
- show coverage;
- edit a draft with placeholders preserved;
- publish through version/ETag protection.

Clarify that product catalogs are source-controlled and CI-checked, while workspace content translations are tenant-owned. User-entered notes are not silently translated.

### 4. Customer path

Open Contacts. Demonstrate keyboard navigation, server-driven list loading, sort/filter, column state, and bulk selection without claiming the browser loaded the entire dataset. Create a synthetic contact:

- First name: `Mira`
- Last name: `Example`
- Email: `mira@example.invalid`

Open details and show the timeline. Create `Example Systems` under Companies and link it to the contact. Demonstrate tags/custom fields or a saved view if the current smoke suite verifies them.

### 5. Sales path

Create a synthetic deal for the company using an integer/minor-unit-backed amount and ISO currency. Open the pipeline and move it one stage. Explain the optimistic UI, version conflict/rollback behavior, bounded per-stage loading, and recorded stage history.

### 6. Collaboration

Create a follow-up task related to the deal, assign a due date/priority, and complete it. Show the entity timeline, activity feed, calendar, and ICS export. If SMTP profile is enabled, show Mailpit as an optional adapter rather than a base dependency.

### 7. Search, notification, and audit

Use global search to find `Mira Example`. Show that snippets are plain text and tenant-scoped. Trigger or open a notification and explain SSE heartbeat/reconnect/replay. Open Settings → Audit and find the contact/deal/task events by request/entity context; no secret should appear.

### 8. Reports and automation

Open Reports, change the period, and explain indexed/bounded aggregation. Create a simple enabled automation such as “deal stage changed → create notification/task,” preview it, trigger it once, and show a single execution result. Do not demo an external webhook/email target without authorization.

### 9. Tenant denial

Use the dedicated E2E/integration scenario rather than trying to access another real tenant manually. Show its test artifact and explain the dual guard + forced RLS boundary. Never stage a cross-tenant demo with sensitive data.

### 10. Evidence, not superlatives

Open [`PERFORMANCE.md`](PERFORMANCE.md), the raw bundle artifact, and [`BENCHMARK_METHODOLOGY.md`](BENCHMARK_METHODOLOGY.md). Clearly separate recorded bundle values from pending Lighthouse/k6/container metrics. Show the competitor worksheet with `Not measured` values.

## Screenshot capture

After the current E2E smoke passes, run the screenshot test for each configured viewport:

```bash
pnpm test:e2e -- --grep "captures real application portfolio views"
```

Then review every output for real data, no secrets, no console errors, correct locale/theme, stable loading state, and no debug panels. Copy approved files to the documented paths in [`screenshots/README.md`](screenshots/README.md); never create placeholders that resemble product screenshots.

## Presenter checklist

- Current commit and dirty state disclosed.
- Demo seed is synthetic and `APP_ENV` is development.
- Required routes have passed current smoke tests.
- No secret, cookie, reset token, API key, webhook secret, or real PII is visible.
- No unsupported “fastest,” competitor, production-certification, or scale claim.
- Any failed/pending step is stated and linked to `STATE.md`.
