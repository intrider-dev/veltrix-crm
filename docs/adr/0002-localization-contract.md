# ADR 0002: Locale-neutral contracts and extensible translation catalogs

- Status: Accepted
- Date: 2026-07-21

## Context

The product must ship in Russian and English and make later language additions, user/workspace locale changes, content templates, API messages, and notification delivery safe and maintainable.

## Decision

- English is the source catalog and Russian is required at equal completeness.
- API problems expose stable `code`, structured `params`, and optional trace metadata. The frontend translates codes; the server does not make authorization or client logic depend on prose.
- User locale overrides workspace locale; workspace locale overrides deployment default. Locale values are validated BCP 47 tags from an allowlist.
- Angular loads small shell/auth catalogs eagerly and lazy feature catalogs with their routes. Catalog keys are generated/type-checked and CI validates missing, extra, and placeholder-mismatched messages.
- Database stores translatable product content as a stable template key plus typed parameters. User-entered CRM content remains unchanged. Email/webhook/in-app renderers select locale at delivery time and record template version, not sensitive rendered content, in audit metadata.
- Dates, numbers, currencies, plurals, and lists use native `Intl`; no Moment.js or remote locale assets.
- A documented extraction/check/add-locale workflow is a release gate.

## Consequences

- Language switching can be immediate for loaded UI and persisted as a user setting.
- Background messages remain consistent and replayable across locale changes.
- Translators edit isolated catalogs without touching components.
- Catalog completeness and placeholder compatibility add CI work but prevent runtime language gaps.
