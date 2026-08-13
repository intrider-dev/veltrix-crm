# ADR 0020: Veltrix Signal design system

- Status: Accepted
- Date: 2026-07-31

## Context

The current interface is functionally broad but visually resembles a generic blue-gray administration template. Shared Material styling improved individual controls, yet route-specific layout, native controls, dense outlined panels, the assignment form, record navigation, and the communication dock still lack one coherent interaction system.

The product owner selected the public FluxCRM dashboard UI Kit as a desired visual direction. The name is ambiguous across several unrelated products; the reviewed reference is the dpopstudio portfolio concept linked in `docs/DESIGN_RESEARCH.md`.

## Decision

Adopt **Veltrix Signal**, an original design system specified in the root `DESIGN.md`.

The system will:

- use a light neutral workspace with deep evergreen and lime semantic accents;
- retain Angular Material/CDK/Aria as the behavioral foundation;
- centralize semantic color, typography, spacing, radius, density, and motion tokens;
- standardize shell, controls, toolbars, data views, record layouts, assignments, and messenger components;
- use action-centered CRM information architecture derived from official CRM documentation;
- meet WCAG 2.2 AA and the existing EN/RU localization contract;
- keep complex data, messenger, media, calls, reports, and grid code lazily loaded.

FluxCRM remains a mood reference only. No logo, brand asset, exact palette, distinctive card geometry, or proprietary typography will be copied.

## Consequences

### Positive

- The product gains a distinct and consistent visual identity.
- Shared primitives replace page-specific layout fixes.
- Preview and action-centered record patterns reduce navigation cost.
- Messenger and media states become explicit product contracts rather than incidental UI.
- Accessibility, localization, and responsive behavior are reviewed at the component level.

### Trade-offs

- A real redesign requires staged replacement of shared primitives before feature polish.
- Visual regression screenshots must be intentionally renewed after the system is implemented.
- The current logo palette may need a centrally configured color revision to fit the new anchor/signal palette.
- Light and dark themes require independent semantic contrast verification.

## Rejected alternatives

### Copy the FluxCRM concept directly

Rejected because it would copy distinctive visual work, does not cover the product's complete CRM workflows, and contains presentation-first layouts that are unsuitable for high-density operations.

### Add a second universal UI kit

Rejected because it would duplicate Angular Material behavior, increase bundle and maintenance cost, and create two competing component contracts.

### Continue page-by-page CSS fixes

Rejected because recurring toolbar, select, form, assignment, and messenger defects originate in missing shared primitives and layout contracts.
