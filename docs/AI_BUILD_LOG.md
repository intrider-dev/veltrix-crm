# AI-assisted build log

This log contains verifiable repository facts, not hidden chain-of-thought, fictional dialogue, or invented work history.

## Source brief

On 2026-07-21 the user supplied a detailed requirement for a production-oriented, open-source, multi-tenant Sales CRM: Angular 22 SPA, Go modular monolith, PostgreSQL 18, app guards plus RLS, a two-container base profile, real CRM workflows, multilingual RU/EN behavior, tests, benchmarks, security hardening, and portfolio documentation. The complete normalized brief is preserved in [`MASTER_PROMPT.md`](MASTER_PROMPT.md).

The repository was initially empty and Git was initialized on `main`. At the time this entry was written, the initial work had not yet been committed; `git status` therefore showed the created repository tree as untracked rather than providing per-file authorship history.

## Skills requested and used

- The marketing skills repository was added project-locally. The `product-marketing` skill was used to create `.agents/product-marketing.md`, aligning the value proposition, audience, wording constraints, evidence language, and README/case-study narrative.
- The design-engineering skill from Emil Kowalski's skills repository was added project-locally. It informed restrained motion, immediate feedback, keyboard/focus details, reduced-motion handling, and avoidance of decorative performance cost.

Installed project skill paths:

- `.agents/skills/product-marketing/`
- `.agents/skills/emil-design-eng/`

## Work stages

| Stage                 | Verifiable output                                                                                                                                                  |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Discovery/planning    | `AGENTS.md`, execution/state/master documents, product context, ADRs, Git main branch                                                                              |
| Contracts/foundations | Central product config, OpenAPI, generated TypeScript/Go config, i18n catalogs/checkers, PostgreSQL migrations and typed SQL                                       |
| Vertical CRM slice    | Identity/workspace, dashboard, contacts/companies, deals/pipeline, activities/audit, Angular routes, demo/small seed                                               |
| Core/advanced domains | Customer bulk/CSV/merge, sales extensions, collaboration/calendar/reporting, notifications/SSE, automation, jobs/outbox, API keys/webhooks, files, MFA/invitations |
| Deployment/quality    | Multi-stage scratch image, Compose profiles, health checks, CI workflows, unit/integration/browser/load scenarios                                                  |
| Portfolio             | Bilingual README/architecture/case study, performance/methodology, threat model, demo/release guidance                                                             |

Subsequent verified slices added projects/tasks and assignment visibility, direct/group chat with optional LiveKit calls, list/Kanban/Gantt deal views, configurable localized lead/deal stages, per-stage role overlays, calendar audiences, and a personal IMAP/SMTP mailbox. Runtime/browser evidence remains governed by `STATE.md` rather than inferred from source presence.

## Independent analysis and review

Parallel agents were assigned bounded architecture, performance, security, UX/accessibility, QA, and later implementation/review tasks. The primary agent retained integration responsibility. Parallel agents shared the working tree, so generated/query/router changes required final formatting and combined verification after handoff.

## Checks and artifacts observed during the build session

- The current bundle reporter produced `benchmarks/results/bundle-report.json` on 2026-07-22. It recorded 92,243 initial Brotli bytes, a 170,601-byte lazy AG Grid Community chunk, a 116,990-byte optional lazy LiveKit chunk, and no external font reference.
- PostgreSQL 18 integration work was run against a local PostgreSQL 18.4 test instance when Docker Desktop was unavailable. A focused RLS isolation suite passed after migration fixes; a workspace-creation test then exposed a separate RLS/context-order issue and triggered a hardening migration/rerun cycle.
- The combined `pnpm check` passed format/lint/vet, complete 967-key EN/RU catalogs, strict TypeScript, Go and Angular tests, the production frontend, precompression, and the embedded Go binary. The serial real-PostgreSQL integration suite also passed through migration `000035`.
- The final running production-like image passed 102/102 Playwright tests across desktop/tablet/mobile and produced the retained portfolio screenshots under `docs/screenshots/`.
- Lighthouse, three browser-performance runs, and two generations of three-clean-run 50-VU k6 evidence were executed. The final post-optimization result, including the mobile-LCP and read/write-p95 misses, is in [`PERFORMANCE.md`](PERFORMANCE.md).
- The scratch runtime export contained no Node.js artifact and used UID/GID `65532:65532`. Production dependency audit reported zero advisories; `govulncheck` found no reachable symbol/package vulnerability.

## Problems found and corrections made

- PostgreSQL schema creation under the NOLOGIN owner needed database-level privileges in a fresh database.
- A PL/pgSQL custom-field validator used an invalid `IF CASE` expression; the boolean expression was corrected and reapplied.
- A workspace RLS membership policy had an incorrect correlation, and workspace creation needed transaction-local tenant context before `INSERT ... RETURNING`; a security-hardening migration was added.
- Docker Desktop's WSL engine returned HTTP 500 and an RCU stall. The build did not fabricate Docker metrics; a local pinned PostgreSQL server was used only for database tests until Docker recovered.
- Parallel Go edits temporarily caused formatting/type mismatches. The final combined formatter/tests are an explicit delivery gate.
- A dated bundle result was not labeled release-final after later source changes.
- Internal mailbox persistence states initially differed from the public OpenAPI vocabulary; explicit mapping tests now keep database workflow states out of the client contract.
- The chat composer, company/contact toolbars, select chevrons, button icon alignment, field spacing, and segmented controls were inconsistent. A shared Material component layer was applied and the updated layouts were exercised in the final browser matrix.
- RLS caused a broad global-search query to avoid the FTS index and scan roughly 300,000 documents. A membership-checked tenant-bound function restored the indexed plan. A later dashboard statement planned/JIT-compiled an unused stage-authorized fallback and caused a measurable full-suite regression; the stored-summary and fallback paths were split, then migration `000035` precomputed search authorization facts and bounded candidates. Two diagnostic reruns and a final three-run baseline retained both improvements and remaining misses without weakening thresholds.
- The seed runner initially rejected a valid restart after E2E-created rows changed live table counts. The immutable seed ledger contract was separated from mutable live CRM data; unit tests, rebuilt-container restart, and the full browser matrix then passed.
- Browser heap measurements were initially distorted by lazy/JIT warm-up. The scenario added ten unmeasured warm-up cycles and lifecycle counters; the final three 20-cycle runs measured 4.0%, 8.3%, and 8.6% retained-heap growth.
- The penultimate browser suite found that the horizontally scrollable Kanban region was not keyboard-focusable in Safari. The production component received a localized accessible name and focus target; the rebuilt final image then passed all 102 browser tests.
- A final QA review found that the retained one-shot setup container violated the exact two-container base-profile requirement. Bootstrap was moved into a PostgreSQL-derived image wrapper that completes checksummed migration/seed work before health readiness; clean-volume and existing-volume runs both retained only `postgres` and `app`.
- The expanded invitation/chat browser flow found that a native member select read only through a template reference did not update a disabled button in zoneless Angular. Explicit signal-backed selection fixed the real interaction, and the two-user message flow then passed in the complete 102-test matrix.
- A security review found the application master-encryption key colocated with the PostgreSQL bootstrap environment. A narrow `LoadBootstrap` path, secret-free database service configuration, sanitized final `postgres` PID 1, fast bootstrap shutdown, and non-recreating one-shot migration scripts closed the finding. Clean/existing-volume runs and independent re-review found no remaining Critical/High security blocker.
- The first final E2E rerun on alternate port `18081` correctly rejected its module request because `APP_PUBLIC_URL` still defaulted to `:8080`. Compose now derives its default public URL from `APP_PORT`; the rebuilt image then passed 102/102 tests in 2.7 minutes.
- Visual inspection of the rebuilt login page moved password recovery below the password control and exposed an adjacent mid-width product-name word break. Local typography containment plus a three-viewport geometric regression fixed both without adding motion or changing the page composition.
- A persistent-browser reproduction showed that an older Angular Service Worker could keep the previous login template and request removed hashed chunks after deployment. The app now checks for updates on startup, activates ready versions automatically on authentication routes, and offers a persistent localized reload action inside the authenticated shell.
- A later leads report was reproduced against a container built before the pending workspace changes. Rebuilding the production image exposed the current list/Kanban/Gantt and details routes; the lead store was then separated into load and mutation/form error channels, and Kanban was changed to bounded per-stage API requests.
- The first message in a newly created lead discussion returned a server error because the shared send path required two recipients even though an entity discussion can initially contain only its authorized opener. The bounded recipient rule now accepts one member for entity discussions, retains the exact two-member rule for direct chats, and has a regression unit test plus a real browser send check.
- Independent review found that entity-chat membership could outlive the user's lead/deal or stage permission. Migration `000037` now re-evaluates record visibility in the RLS predicates and recipient query, supports least-privilege `leads.read`/`deals.read` routes, caps atomic self-join, and has a real PostgreSQL regression proving revoked users neither read messages nor receive SSE.
- The first record-discussion submit could clear a draft before asynchronous conversation resolution completed. Sending now returns an explicit success result, composer state clears only after success, and attachment metadata/media/recording work is bounded to the current page, a single requested blob, five minutes, and 25 MiB.
- Lead/deal custom fields were validated in their JSON aggregates but were not consistently persisted to the normalized custom-field value table used for schema-change safety. Mutations now update both representations transactionally; migration `000037` backfills existing values and an integration test covers incompatible type changes plus aggregate cleanup on definition deletion.
- A dedicated production-browser regression now covers the reported lead workspace end to end: create, List/Kanban/Gantt, stage transition, details save, and first record-discussion message with browser console/page-error capture.
- Independent QA/security/maintainability review exposed ambiguous media retry, provisional-delete replay, stale conversation refresh, and async LiveKit cleanup boundaries. Stable idempotency keys, advisory-lock replay, generation guards, server-side call release, and focused frontend/real-PostgreSQL regressions closed those findings.
- A real PostgreSQL 18 attachment integration run exposed an invalid `{1,500}` regular-expression repetition in the original storage-key constraint. Migration `000042` replaced it with an equivalent length plus character-whitelist constraint and keeps rollback executable on PostgreSQL 18.
- The final production-like image was rebuilt at `http://127.0.0.1:18081`; migrations through `000042`, health endpoints, non-root scratch runtime, Node.js absence, bundle report, and the leads List/Kanban/Gantt/details/discussion browser flow were rechecked rather than inferred.
- The first browser run immediately after a later cold rebuild exposed a Signal Form readiness race: the lead form looked submittable while client validation could still suppress POST without feedback. The create action now waits on real form validity, marks fields touched, and renders localized name/email errors; a new cold-image run passed the complete lead workspace scenario.

## AI-assisted file inventory

Because the workspace began empty, all initial repository content was created or generated with AI assistance in this session. The major groups are:

- root governance/deployment: `AGENTS.md`, `README*.md`, license/community/security/roadmap/changelog files, `Dockerfile`, `compose.yaml`, workspace/tool configuration;
- application source: `apps/api/**` and `apps/web/**`;
- shared configuration/contracts/catalogs: `packages/**`;
- test/benchmark/operations: `benchmarks/**`, `scripts/**`, `infra/**`, `playwright.config.ts`;
- documentation and decisions: `docs/**`;
- GitHub automation/templates: `.github/**`;
- requested project-local skills/context: `.agents/**`.

Generated files—such as lockfiles, OpenAPI TypeScript types, brand/i18n generated types, and `sqlc` output—were produced by their respective tools from AI-assisted source definitions. This inventory does not claim that AI owns third-party dependencies, tool output licenses, or the upstream skill repositories.

## Truthfulness policy

- Test source is not described as a passing test until its command succeeds.
- Compose limits are not described as observed memory/CPU.
- No screenshot is created from a mock and labeled as the real product.
- No customer, testimonial, competitor measurement, or production certification is claimed.
- Final state, known limits, command evidence, and actual metrics are maintained in [`STATE.md`](STATE.md) and [`PERFORMANCE.md`](PERFORMANCE.md).
