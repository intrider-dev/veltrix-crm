# Localization architecture

VeltrixCRM is multilingual at three different boundaries. Keeping them separate prevents a product release, a user's preference, and a workspace's customer-facing copy from overwriting one another.

## Resolution model

| Boundary                                      | Source of truth                         | Resolution order                                                              |
| --------------------------------------------- | --------------------------------------- | ----------------------------------------------------------------------------- |
| Product interface and built-in security email | Version-controlled RU/EN catalogs       | user locale → workspace default → deployment default (`en`)                   |
| Dates, numbers, money                         | Native `Intl` APIs                      | active user locale + workspace IANA timezone                                  |
| Workspace-owned templates and messages        | PostgreSQL localization tables with RLS | requested published translation → workspace default translation → source text |

User-entered CRM records are not translated implicitly. Contact names, notes, activities, custom field values, and imported data remain exactly as entered. This avoids silent data mutation and makes external AI translation impossible without an explicit future feature and consent model.

## Interface catalogs

The canonical source lives in `packages/i18n/source/en`. Each JSON file is one lazy-loadable namespace. Complete translations live in `packages/i18n/locales/<locale>`; locale metadata is in `packages/i18n/locale-manifest.json`.

`pnpm generate:i18n` validates key parity and placeholder parity, then generates:

- the compile-time `MessageKey` union;
- lazy JSON assets for Angular;
- the Go catalog used by RFC Problem Details and built-in email.

Angular loads common namespaces at startup and feature namespaces with route resolvers. Requests are deduplicated, English is a deterministic fallback, and changing language updates `<html lang>` and text direction immediately. Catalogs use system fonts and create no external font requests.

### Adding a message

1. Add a stable semantic key to the smallest English namespace.
2. Add the same key to every locale.
3. Preserve named placeholders exactly. Reordering `{count}` and `{name}` is valid; removing or renaming one is not.
4. Run:

   ```bash
   pnpm generate:i18n
   pnpm check:i18n
   pnpm i18n:extract
   pnpm test:web
   ```

The build fails for missing keys, extra keys, invalid placeholder sets, or unfinished `⟦TODO⟧` values.

### Adding a locale

```bash
pnpm i18n:add-locale --locale pt-BR --name "Português (Brasil)"
```

The scaffolder validates the BCP 47 tag, derives LTR/RTL direction, creates every namespaced catalog, updates the locale manifest and centralized product configuration, and leaves explicit `⟦TODO⟧` values. Translate all values, then run `pnpm generate:brand && pnpm generate:i18n`. A release cannot pass localization checks while TODO markers remain.

## User and workspace settings

The signed-in user's locale is persisted through `PATCH /api/v1/me`; it is not stored as an authentication token or trusted from `localStorage`. A workspace also stores a default locale, its allowed locales, and an IANA timezone. Only installed locales can be enabled. Optimistic versions/ETags prevent two administrators from silently overwriting localization settings.

Preference order is deterministic:

1. the user's supported locale;
2. the active workspace default locale;
3. centralized deployment default;
4. English as the catalog fallback.

The Settings screen changes the user's language immediately. Workspace creation captures locale and timezone. The Translation Center limits translators to languages enabled by that workspace.

## Tenant-owned translated content

The Translation Center at `/settings/translations` manages workspace templates and messages without changing application catalogs. A resource contains a namespace, stable key, source locale, source text, translator description, and extracted placeholders. Each locale has an independent `missing`, `draft`, or `published` translation and version.

Important guarantees:

- every table and query is scoped by `workspace_id` and protected by PostgreSQL RLS;
- source and translation length are bounded;
- namespace, key, and locale tags are validated;
- placeholder sets must match before a translation is saved;
- cursor pagination is bound to its filters;
- updates use `If-Match` and return a conflict instead of losing another translator's edit;
- audit and outbox events contain identifiers and status, not the translated message body;
- only published content is resolved for delivery;
- coverage is reported by namespace so missing content is visible.

Relevant API routes are under `/api/v1/workspaces/{workspaceId}`: `translations`, `translation-coverage`, and `localization-settings`. The OpenAPI 3.1 document is the source for generated Go and TypeScript contract types.

## Email and notifications

Authentication and password-reset email uses checked release catalogs and cannot be overridden by tenant content. Workspace-owned automation/email copy can use tenant translations after explicit publication. Rendering rejects unresolved placeholders and line breaks in email subjects.

## QA checklist

- Run catalog generation and stale-generated-code checks.
- Run unit tests for fallback, plural categories, date/timezone formatting, placeholders, and concurrent translation versions.
- Verify language switching in a real browser and reload the session.
- Run axe against both English and Russian navigation.
- Check long Russian labels at desktop, tablet, and mobile viewports.
- For a new RTL locale, verify direction, focus order, overlays, tables, and bidirectional identifiers before marking the locale supported.

The system intentionally avoids machine-translating CRM data or sending it to an external provider. Such behavior would require a separate opt-in privacy design.
