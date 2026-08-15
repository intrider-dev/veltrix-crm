# ADR 0022: International phone input and canonical storage

Date: 2026-08-15

Status: accepted

## Context

Lead and contact forms previously accepted an unrestricted phone string. Users had no country context, format guidance, or early validation, and equivalent numbers could reach the API in inconsistent national formats. A hand-maintained country/dial-code list would duplicate frequently changing numbering metadata and require recreating country search, flags, keyboard behavior, localization, formatting, and validation.

## Decision

Use the official `@intl-tel-input/angular` and `intl-tel-input` packages at the same pinned stable version for every lead and contact phone field.

- The country picker displays locally bundled flags and supports country-name/dial-code search, keyboard navigation, and accessible labels.
- The active CRM language selects the picker interface and country-name locale. Changing language recreates only the phone control so the library's initialization-only localization options are applied safely.
- The browser locale supplies the initial country when it has a supported region; Russian and English fall back to Russia and the United States respectively. No IP lookup or external request is made.
- The visible number is formatted as the user types in the selected country's national format. The form model and API payload receive E.164.
- A non-empty invalid number makes the Angular Signal Form field invalid and exposes a localized inline error. Empty phone fields remain valid because phone is optional in current CRM records.
- The numbering utility is dynamically imported when a phone control is instantiated. It is not part of the initial application bundle.
- One shared `PhoneInputComponent` integrates the third-party control with design tokens, compact density, dark/light themes, mobile fullscreen selection, form focus/touch state, and the application's EN/RU catalogs.

## Consequences

The application gains two MIT-licensed production dependencies and locally packaged flag assets. Production CSS increases slightly, while the larger numbering metadata remains a lazy chunk. Stored phone values become consistent for new and edited records; existing values are normalized the next time they are edited. Server-side phone normalization remains a defense and import/API clients do not rely on browser validation.

The shared component is the only supported frontend phone-entry path. New entity forms should reuse it rather than importing the library directly.
