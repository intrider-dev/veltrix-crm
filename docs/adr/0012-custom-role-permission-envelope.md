# ADR 0012: Custom roles narrow a server-defined permission envelope

- Status: accepted
- Date: 2026-07-22

## Context

VeltrixCRM originally stored one of five role names on each workspace membership and duplicated the role-to-permission matrix in Go and Angular. Custom roles are required, but most existing tenant-owned RLS policies still use the compatible coarse role name as their database authorization envelope. Allowing an arbitrary custom grant to exceed that envelope would make application and database authorization disagree.

## Decision

Each workspace has immutable system roles and editable custom roles. A custom role selects `admin`, `manager`, `sales`, or `viewer` as its maximum envelope and may only remove permissions from that system role. It cannot use `owner` as a base or receive `roles.write`.

Memberships reference a workspace role by a composite tenant-safe foreign key while retaining the base role string during the staged authorization migration. Effective permissions are loaded inside every tenant transaction after `SET LOCAL` context and are returned by the session API. Angular consumes only these server-provided permissions; it does not reconstruct a role matrix.

Only an active owner has `roles.write`. System roles are immutable. Owner memberships cannot be changed through the custom-role assignment endpoint. Role creation, update, deletion, and assignment are versioned or transaction-locked, audited, tenant-scoped by RLS, and covered by negative PostgreSQL integration tests. A role referenced by a membership cannot be deleted.

## Consequences

- Custom roles are useful immediately for least-privilege subsets of the existing permission catalogue.
- A custom role cannot silently bypass the current RLS envelope.
- New domain and stage permissions must be added fail-closed through migrations and translated into object capabilities before custom roles can broaden beyond the current coarse catalogue.
- The compatibility role string remains temporary and can be removed after every handler, query, report, search path, attachment, and event side channel uses the normalized authorization model.
