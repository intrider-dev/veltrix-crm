# Security policy

VeltrixCRM is in active pre-1.0 development. Only the latest commit on `main` is currently eligible for security fixes. This policy does not certify an unpublished build for production use.

## Report a vulnerability

Use **Report a vulnerability** on the repository's Security tab. If private vulnerability reporting is unavailable, contact the repository owner privately through the address shown on their GitHub profile.

Do not open a public issue for an unpatched vulnerability. Do not include credentials, secrets, or real CRM records in the initial report.

Include, with synthetic data where possible:

- the affected commit or release;
- the impact and attacker prerequisites;
- minimal reproduction steps;
- the affected boundary, such as authentication, authorization, tenant isolation, uploads, webhooks, or data integrity;
- sanitized logs and request IDs;
- a suggested mitigation, if known.

Receipt will be acknowledged, severity and affected versions will be assessed, and disclosure will be coordinated with the reporter. Timing depends on impact and maintainer availability; no fixed response SLA is promised.

## Research boundaries

- Test only systems and workspaces you own or have explicit permission to assess.
- Stop after demonstrating the minimum necessary impact.
- Do not use persistence, denial of service, social engineering, or automated access to third-party CRM systems.
- Do not access another person's data. Delete any data obtained accidentally.
- Keep reproduction details private until a disclosure date is agreed.

Good-faith research within these boundaries will not be treated as malicious by the maintainers. This statement does not replace authorization from a deployment owner or waive applicable law.

## Security model

The base deployment uses a same-origin Angular SPA, one Go application, and PostgreSQL. Important boundaries include:

- application authorization plus forced PostgreSQL RLS for tenant-owned data;
- transaction-local workspace context on pooled database connections;
- Argon2id password hashing and hash-only storage for session, recovery, and API secrets;
- HttpOnly cookies, CSRF protection, request limits, secure headers, and same-origin operation by default;
- scoped API keys, signed webhooks, upload validation, encrypted mailbox credentials, and short-lived call grants;
- authenticated, TLS-verified Kafka/RabbitMQ connections in production, strict PII-free broker envelopes, static operator-owned routing, bounded producers, and at-least-once consumer idempotency;
- append-oriented audit records that omit passwords, tokens, and secrets.

Security controls and checks continue to evolve before 1.0. Consult [docs/STATE.md](docs/STATE.md), [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md), and [.github/SECURITY_REVIEW_CHECKLIST.md](.github/SECURITY_REVIEW_CHECKLIST.md) for current evidence and open limitations. A clean scanner result alone is not proof of security.
