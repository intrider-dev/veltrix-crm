# ADR 0014: Leads, deals, and funnel configuration have independent permissions

- Status: accepted
- Date: 2026-07-22

## Context

The original `records.*` permission family could not express a role that reads leads without deals or allow funnel configuration without granting unrelated workspace settings. The user interface could hide controls, but server handlers still needed a fail-closed authorization distinction.

## Decision

The permission catalogue includes independent `leads.read/create/update/delete`, `deals.read/create/update/delete`, `lead_stages.manage`, and `deal_stages.manage` grants. Existing system and custom roles are migrated from their current record envelope to preserve behavior. Every lead, deal, conversion, Kanban move, pipeline, and stage HTTP handler checks the specific permission inside the tenant transaction. Angular consumes the effective server grants and disables or removes actions consistently, but server authorization remains authoritative.

Stage lifecycle configuration is an administrative permission distinct from moving an authorized record between configured stages. Object assignment visibility and PostgreSQL RLS remain an additional layer and cannot be broadened by the UI permission gate.

## Consequences

- A custom role can be lead-only or deal-only without receiving the other resource.
- Funnel administrators no longer need the broad `settings.write` grant.
- Adding per-stage transition rules later can build on explicit stage permissions rather than overloading record CRUD.
- Migration and integration tests must prove that legacy role behavior is preserved and custom-role narrowing is enforced.

