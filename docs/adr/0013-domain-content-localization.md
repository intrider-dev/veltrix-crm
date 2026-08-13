# ADR 0013: Domain-owned labels use versioned workspace content resources

- Status: accepted
- Date: 2026-07-22

## Context

VeltrixCRM separates product-interface messages from tenant-authored content. Configurable pipeline and stage names are authored by workspace administrators, but they are also navigation and reporting labels that must be understandable in every workspace language. Copying them into static Angular catalogs would make tenant changes impossible; storing one translated column per locale would make each new locale a schema migration.

## Decision

Pipeline, deal-stage, and custom lead-stage names remain canonical source fields in their owning sales tables. The application registers each source name in `localization.content_resources` within the same tenant transaction as the domain mutation. The stable resource key is the UUID and the namespaces are `sales.pipeline.name`, `sales.pipeline_stage.name`, and `sales.lead_stage.name`.

Read APIs return both `name` (the editable source) and `displayName` (the published translation resolved for membership locale, then user locale, then workspace default). Resolution is one bounded SQL query per namespace, not one query per record. Built-in lead-stage labels keep stable system keys and use the application catalog.

Changing source locale, source text, or placeholders increments the resource version and returns every published translation for that resource to `draft`. Deleting a domain record removes its translation resource in the same transaction. Translation authors continue to use the existing draft/publish workflow and optimistic versions in the translation center.

## Consequences

- Tenant-authored navigation labels can be translated without schema changes or frontend releases.
- Editors always modify the source value while ordinary views display the effective locale.
- A renamed source cannot silently retain a semantically stale published translation.
- List endpoints add at most a small fixed number of localization queries and do not create N+1 load.
- Existing records created outside application services need an explicit deterministic backfill before their names appear in the translation center.

