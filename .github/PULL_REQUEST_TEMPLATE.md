## Outcome

Describe the user-visible or operational outcome and the scope deliberately left unchanged.

## Verification

List exact commands and results. Use “Not run” with a reason for every relevant check that was not executed.

- [ ] `pnpm lint`
- [ ] `pnpm typecheck`
- [ ] `pnpm test`
- [ ] `pnpm test:integration` (when database behavior changes)
- [ ] `pnpm test:e2e` (when a user flow changes)
- [ ] `pnpm build` (when production code or assets change)

## Risk review

- [ ] Tenant-owned SQL starts compound indexes with `workspace_id` and retains forced RLS.
- [ ] Object authorization and negative cross-tenant tests cover new data access.
- [ ] No credential, token, PII, or customer data is present in code, logs, fixtures, or screenshots.
- [ ] New user-visible strings use typed i18n keys; English and Russian catalogs remain complete.
- [ ] API changes update OpenAPI-generated contracts without manual DTO drift.
- [ ] Duplicate-prone mutations, concurrency, and retry behavior are handled where applicable.
- [ ] Accessibility and keyboard behavior were checked for changed UI.
- [ ] Performance-sensitive changes include comparable evidence or explain why measurement is not applicable.
- [ ] Documentation and `docs/STATE.md` describe the actual state without unverified claims.

## Evidence

Add sanitized screenshots, traces, bundle reports, query plans, or benchmark artifacts when applicable.
