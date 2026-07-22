# ADR 0019: Database administration is a one-shot deployment phase

- Status: accepted
- Date: 2026-07-22

## Context

The request-serving binary previously received database-administrator credentials so it could create roles and migrate at startup. That credential would remain available for the entire application lifetime and could defeat PostgreSQL RLS if the process were compromised.

## Decision

The same production binary exposes a finite `bootstrap` command that creates application roles, validates migration checksums, applies forward migrations, and optionally creates the documented development seed. A custom image based on the pinned PostgreSQL image contains a copy of that migration binary. Its entrypoint waits for the official PostgreSQL initialization to reach the final TCP listener, runs `bootstrap` as the local `postgres` user, and creates the readiness marker only after success. Compose then starts the unprivileged application service. The base profile therefore creates exactly two containers, while the application receives only the `veltrix_app` URL and no administrator URL, role password, automatic-migration flag, or demo password.

Migration contents and names are recorded with SHA-256 checksums. Existing ledgers are backfilled once; after that baseline, edited historical migrations fail closed. Corrective schema work uses a new forward migration rather than changing an already published migration.

## Consequences

- The base profile contains exactly one application container plus PostgreSQL; there is no retained setup container.
- PostgreSQL cannot become healthy, and the application cannot start, until the checksummed migration/bootstrap phase succeeds.
- A fresh database needs administrator credentials only inside the database container, which already owns the cluster; the serving process cannot use them.
- The PostgreSQL image contains a dormant copy of the migration binary after startup. It is not listening or running, but image provenance and database-container access remain privileged operator boundaries.
- The first checksum backfill cannot reconstruct whether a migration was edited before checksums existed, so release provenance still matters.
