# ADR 0005: Lazy Community grid, custom SVG charts, and embedded precompressed assets

- Status: Accepted
- Date: 2026-07-21

## Context

CRM list screens need selection, keyboard navigation, filtering, sorting, column state, resizing, and bounded rendering. Dashboards need a small number of charts. Adding these capabilities must not inflate every route or add runtime infrastructure, fonts, or licensing surprises.

## Decision

- Use AG Grid Community only for complex list features. Import it exclusively from lazy routes, register only used Community modules, use server cursor/infinite data, and forbid `ag-grid-enterprise` in dependency/import checks.
- Use Angular Material as the only general UI kit and CDK/Aria for behavior. Do not add Tailwind or another universal component system.
- Implement current report visuals as small accessible SVG components using semantic labels and design tokens. Reconsider a community chart library only when a concrete chart cannot be implemented clearly and an ADR compares compressed route cost/accessibility/maintenance.
- Use system fonts and selective inline/code-native SVG icons; make no external font or icon-font request.
- Build the Angular SPA in a Node-only stage, precompress emitted assets with deterministic gzip/Brotli, copy them into the Go embed tree, and ship a static non-root runtime without Node.js.
- Apply immutable caching to fingerprinted assets, negotiate `Accept-Encoding`, and preserve SPA fallback without masking API 404s.
- Measure emitted output independently: initial JS+CSS targets 350 KiB Brotli, warns above 400, fails above 450; ordinary lazy features target 200 KiB. Large grid/report chunks must remain lazy and documented.

## Evidence considered

The dated 2026-07-21 bundle artifact recorded 84.7 KiB initial JS+CSS Brotli and a 154.4 KiB lazy AG Grid chunk. It showed no external-font reference. Later source changes require a release rerun, so this evidence validates the approach only for that historical build.

## Consequences

- Basic routes do not pay the grid cost; complex lists still receive mature keyboard/virtualized behavior.
- Custom SVG charts minimize payload but offer fewer advanced chart features and require direct accessibility testing.
- Precompression increases build time/storage slightly and requires correct variant/cache headers.
- Hashed chunks change each build, so `stats.json` is needed to map bundle artifacts back to routes.
- Community-only licensing and forbidden-import checks are release gates.

## Alternatives rejected

- **Native table for every complex list:** substantial behavior/accessibility/virtualization code and maintenance risk.
- **AG Grid Enterprise:** prohibited scope and commercial dependency.
- **Eager grid import:** charges login/dashboard users for unused functionality.
- **General chart library now:** no current capability justifies its route payload.
- **CDN fonts/icons:** adds requests, privacy/availability dependency, and layout variability.
- **Node static server in production:** duplicates the Go serving path and increases runtime components.
