# Security policy

Security defects, especially tenant-isolation or authentication failures, should be reported privately. Do not open a public issue before maintainers have had a reasonable opportunity to investigate and release a fix.

## Supported versions

The project is currently in pre-release development. Until the first tagged stable release, only the latest commit on `main` is eligible for security fixes. A version support table will be added when releases exist; this file does not imply that an unpublished build is production-certified.

## Private reporting

Use GitHub’s **Report a vulnerability** form under the repository Security tab (private vulnerability reporting). If that feature is unavailable, contact the repository owner privately through the contact method shown on their GitHub profile. Do not send secrets or real CRM records in the initial report.

Include, using synthetic data where possible:

- affected commit or release;
- impact and attacker prerequisites;
- minimal reproduction steps;
- whether tenant boundaries, authentication, authorization, uploads, webhooks, or data integrity are involved;
- sanitized logs and request IDs;
- a proposed mitigation, if known.

Maintainers will acknowledge receipt, assess severity and affected versions, coordinate a fix and disclosure, and credit the reporter if requested. Exact response or release timing depends on impact and maintainer availability, so no fixed SLA is claimed.

## Disclosure expectations

- Keep reports and proof-of-concept details private until a coordinated disclosure date is agreed.
- Test only systems and workspaces you own or are explicitly authorized to assess.
- Avoid persistence, denial of service, social engineering, automated third-party CRM access, and access to other users’ data.
- Stop after demonstrating the minimum impact and delete any accidentally obtained data.

Good-faith research that follows these constraints will not be treated as malicious by project maintainers. This statement is not a substitute for authorization from a deployment owner and does not waive applicable law.

## Project security model

The base deployment uses a same-origin Angular SPA, one Go application binary, and PostgreSQL. Tenant isolation is enforced by application authorization and forced PostgreSQL RLS with transaction-local context. Sensitive tokens are stored as hashes, production sessions use secure cookies, and the default deployment does not require Redis, a broker, or an external search service.

The implementation and CI are still subject to verification. See `docs/STATE.md`, `docs/THREAT_MODEL.md`, and `.github/SECURITY_REVIEW_CHECKLIST.md` for current evidence and open limits; absence of a scanner finding is not proof of security.
