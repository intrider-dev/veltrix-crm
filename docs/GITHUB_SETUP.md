# GitHub publication setup

Use the central product configuration as the canonical naming source. The text below is proposed metadata; applying repository settings, creating a release, or pushing is intentionally outside the local build task.

## Repository metadata

**Recommended description**

> Resource-conscious, multi-tenant Sales CRM: Angular 22, Go, PostgreSQL RLS, RU/EN, reproducible benchmarks, and a two-container base profile.

**Recommended topics**

`crm`, `sales-crm`, `angular`, `angular-material`, `golang`, `postgresql`, `multi-tenant`, `row-level-security`, `openapi`, `server-sent-events`, `pwa`, `i18n`, `performance`, `self-hosted`, `open-source`

**Website**

Leave blank until a maintained public demo or documentation site exists. Do not point to a local or unsecured deployment.

## Repository settings

- Default branch: `main`.
- Enable Issues, Discussions only if a maintainer can moderate them, private vulnerability reporting, Dependabot alerts/security updates, secret scanning, and push protection where available.
- Protect `main`: require pull request, current CI checks, conversation resolution, linear history if desired, and no force pushes/deletions.
- Require signed commits only if every contributor can meet that policy; document it before enforcement.
- Disable merge strategies the maintainers do not intend to support; squash merge fits Conventional Commit release notes.
- Add the CodeQL default setup only if it does not duplicate the committed workflow.
- Configure Actions with least-privilege default token permissions and pinned approved actions.

## Publication order

1. Finish combined local checks and update `docs/STATE.md`/`docs/PERFORMANCE.md` with exact artifacts.
2. Make logical Conventional Commits; ensure generated code and lockfiles are clean.
3. Push `main` without changing history and wait for all required hosted workflows.
4. Triage scanner/license findings; fix critical/high issues and rerun affected jobs.
5. Run the manual full benchmark on the release commit and retain artifacts.
6. Capture real Playwright screenshots and add only approved, sanitized images.
7. Update README/case study metrics and screenshot status from those artifacts.
8. Create an annotated `v0.1.0` tag only when the release checklist is complete.
9. Publish the release with checksums/SBOM/container reference if an image is published.
10. Verify quick start from a clean clone on a second environment.

## Proposed first release

**Title:** `v0.1.0 — first reproducible CRM release`

**Draft release text:**

> This is the first evidence-bearing release of the resource-conscious, multi-tenant Sales CRM. The base deployment combines one non-root Go application container with PostgreSQL 18. It includes the documented Sales CRM workflows, RU/EN localization, application authorization plus forced RLS, PostgreSQL search/jobs/outbox, an Angular 22 SPA, and reproducible test/benchmark protocols.
>
> Verified for this tag:
>
> - [ ] clean clone and `docker compose up --build`;
> - [ ] clean migrations and deterministic small seed;
> - [ ] backend/frontend/integration/E2E checks;
> - [ ] tenant-negative and security checks;
> - [ ] production image and runtime-no-Node verification;
> - [ ] release-final bundle, Lighthouse, k6, and Docker resource artifacts;
> - [ ] real sanitized screenshots.
>
> Actual values and host/profile details: `docs/PERFORMANCE.md`. Known limitations: `docs/STATE.md`. Security reporting: `SECURITY.md`.

Remove the checkboxes only by replacing them with exact successful evidence. Do not publish this draft while required entries remain unchecked.

## Social preview and screenshots

Recommended 1280×640 social preview: a real desktop dashboard crop next to a compact architecture statement (“Angular SPA · one Go binary · PostgreSQL”), created only after the application screenshot is captured. Keep text inside GitHub's safe center area and use the actual light/dark design tokens.

Publication set:

1. dashboard overview;
2. contacts grid with synthetic rows;
3. deal pipeline;
4. contact/company details timeline;
5. reports;
6. dark-theme dashboard;
7. optional narrow mobile navigation.

Source/acceptance instructions are in [`screenshots/README.md`](screenshots/README.md). Do not publish Figma/mock/generated UI as product evidence.

## Suggested Conventional Commits

Use only the messages matching the final diff; split large initial work if practical:

- `chore(repo): establish workspace and centralized product config`
- `feat(api): add tenant-safe crm modular monolith`
- `feat(auth): add secure sessions workspaces rbac and mfa`
- `feat(web): add zoneless multilingual crm workspace`
- `feat(customers): add advanced records csv and saved views`
- `feat(sales): add leads pipelines deals and stage history`
- `feat(collaboration): add activities calendar notifications and reports`
- `feat(automation): add durable rules jobs and webhook delivery`
- `feat(files): add streaming local and s3-compatible attachments`
- `build(container): add non-root embedded spa production image`
- `test: add tenant integration browser and benchmark scenarios`
- `docs: add bilingual portfolio security and performance evidence`
- `ci: add quality security container and benchmark workflows`

## Release gates

- No unsupported product/competitor superlatives.
- No `Not measured` cell presented as a pass.
- README and case study agree with `STATE.md` and `PERFORMANCE.md`.
- Screenshots are real and sanitized.
- Demo credentials are development-only and demo seed is off in production guidance.
- `packages/product-config/product.json` matches repository URLs before publication.
- License, security contacts, support capacity, and roadmap language are accurate.
- No remote has been changed and no push/release occurs without repository-owner intent.
