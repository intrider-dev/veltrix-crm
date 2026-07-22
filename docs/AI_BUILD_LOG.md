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

## Independent analysis and review

Parallel agents were assigned bounded architecture, performance, security, UX/accessibility, QA, and later implementation/review tasks. The primary agent retained integration responsibility. Parallel agents shared the working tree, so generated/query/router changes required final formatting and combined verification after handoff.

## Checks and artifacts observed during the build session

- The bundle reporter successfully produced `benchmarks/results/bundle-report.json` on 2026-07-21. It recorded 86,727 initial Brotli bytes and no external font reference in emitted HTML/CSS/JS. Later frontend edits make a release rerun mandatory.
- PostgreSQL 18 integration work was run against a local PostgreSQL 18.4 test instance when Docker Desktop was unavailable. A focused RLS isolation suite passed after migration fixes; a workspace-creation test then exposed a separate RLS/context-order issue and triggered a hardening migration/rerun cycle.
- Go and frontend format/lint/typecheck/unit/build commands were run repeatedly during implementation. Authoritative final command lines and outcomes must be copied to [`STATE.md`](STATE.md) only after the combined tree is stable.
- Playwright, Lighthouse, k6, production-image resource measurement, and final screenshots are not claimed by this entry without final artifacts.

## Problems found and corrections made

- PostgreSQL schema creation under the NOLOGIN owner needed database-level privileges in a fresh database.
- A PL/pgSQL custom-field validator used an invalid `IF CASE` expression; the boolean expression was corrected and reapplied.
- A workspace RLS membership policy had an incorrect correlation, and workspace creation needed transaction-local tenant context before `INSERT ... RETURNING`; a security-hardening migration was added.
- Docker Desktop's WSL engine returned HTTP 500 and an RCU stall. The build did not fabricate Docker metrics; a local pinned PostgreSQL server was used only for database tests until Docker recovered.
- Parallel Go edits temporarily caused formatting/type mismatches. The final combined formatter/tests are an explicit delivery gate.
- A dated bundle result was not labeled release-final after later source changes.

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
