# Security review checklist

Use this checklist for security-sensitive changes and record concrete evidence in the pull request.

## Identity and session

- Passwords remain Argon2id-hashed with bounded work and no secret logging.
- Session, reset, recovery, invitation, and API tokens are cryptographically random and stored only as hashes.
- Cookie flags, CSRF, session rotation/expiry, logout, logout-all, lockout, and generic authentication errors are tested.
- MFA enrollment, verification, recovery codes, and secret encryption fail closed.

## Tenancy and authorization

- Every tenant-owned row includes `workspace_id`; compound uniqueness/indexes start with it.
- Runtime roles are not owners, superusers, or `BYPASSRLS`; forced RLS and policies cover every tenant table.
- Tenant context is transaction-local and cannot survive pool reuse.
- Application guards and exact RBAC permissions protect every object operation.
- Negative tests cover read, update, delete, search, export, bulk actions, SSE, jobs, files, and webhooks across tenants.

## Inputs, outputs, and integrations

- SQL is parameterized; request/body/upload/response sizes and decompression are bounded.
- HTML, CSV, filenames, MIME types, paths, redirects, URLs, and webhook destinations are validated for their sink.
- Upload/download is streamed, authorized, traversal-safe, and preserves only a safe display name.
- Webhooks use HMAC, timestamps/replay defense, bounded retries, secret rotation, and redacted logs.
- API keys are scoped, hash-only, revocable, rate-limited, and update last-used without exposing secrets.

## Async and observability

- Jobs/outbox/automation are idempotent, tenant-scoped, bounded, lease-safe, retry-limited, and dead-lettered.
- SSE authorization, replay bounds, backpressure, cleanup, and Last-Event-ID cannot cross tenants.
- Audit events are append-oriented and exclude passwords, tokens, MFA seeds, webhook secrets, and sensitive content.
- Request IDs and structured logs are useful without recording PII or secrets.

## Supply chain and deployment

- Lockfiles and generated sources are clean; no forbidden frontend dependency or Enterprise module was added.
- `govulncheck`, `pnpm audit`, CodeQL, Trivy, license policy, and SBOM workflows remain fail-closed.
- The runtime image is non-root, contains no Node.js, and retains least privilege/read-only filesystem constraints.
- `.env.example`, fixtures, screenshots, traces, and benchmark artifacts contain synthetic data only.
