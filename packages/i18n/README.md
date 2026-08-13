# Localization catalogs

VeltrixCRM separates release-owned interface copy from tenant-owned business content.

- `source/en/*.json` is the canonical interface catalog.
- `locales/<locale>/*.json` contains a complete translation with the same keys and placeholders.
- `locale-manifest.json` stores locale metadata, direction, and native name.
- `apps/web/public/i18n` and the Go catalog are generated artifacts; do not edit them directly.

## Everyday workflow

1. Add or edit the English source message in the smallest relevant namespace.
2. Apply the same key to every locale while preserving named placeholders such as `{count}`.
3. Run `pnpm generate:i18n`.
4. Run `pnpm check:i18n && pnpm i18n:extract`.

Use `pnpm i18n:add-locale --locale pt-BR --name "Português (Brasil)"` to scaffold another locale. Replace every `⟦TODO⟧` value, then run `pnpm generate:brand && pnpm generate:i18n`. The command also registers the locale in centralized product configuration.

Route-level catalogs stay lazy. Keep global navigation and error messages in eager namespaces; feature copy belongs in its feature namespace. The source key is a stable identifier, not prose to be shown to a user.

See [Localization architecture](../../docs/LOCALIZATION.md) for fallback rules, tenant content translations, API behavior, and QA.
