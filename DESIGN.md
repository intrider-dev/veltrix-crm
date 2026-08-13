# VeltrixCRM design system

**Status: implemented and evolving during active development.**

Direction: **Veltrix Signal**

Last updated: 2026-08-13

This document defines the interface rules for VeltrixCRM. New screens should reuse these tokens, components, layouts, and interaction patterns. Visual references inform the product direction but never authorize copying another product's assets, brand marks, or distinctive artwork.

## 1. Product experience

VeltrixCRM is an action-centered sales workspace, not a database browser and not a collection of dashboard cards.

Every primary screen must answer four questions:

1. What changed?
2. What needs attention?
3. What is the next useful action?
4. Who owns it?

The default workflow is:

`workspace -> saved view -> list or board -> preview drawer -> full record`

Use progressive disclosure. A seller should be able to scan and act without opening a full record; complex editing, relationships, files, discussion, and history belong on the full record route.

## 2. Visual direction

### Character

The interface should feel focused, crisp, calm, and purposeful. Its foundation is:

- a light neutral canvas rather than a blue-gray admin template;
- strong black or deep-green typography;
- one vivid lime signal color paired with a deep evergreen anchor;
- large, legible numbers and clear labels;
- rounded but predominantly rectangular controls;
- generous grouping space and restrained borders;
- clean tables and compact contextual toolbars;
- responsive layouts designed as real mobile workspaces.

Avoid:

- its logo, font, icon artwork, card cut-outs, diagonal card patterns, or exact palette;
- oversized pill navigation for the whole product;
- decorative KPI cards that do not lead to an action;
- low-density layouts that waste space on operational screens;
- controls whose state is communicated only by color;
- financial or inventory concepts that are outside the Sales CRM scope.

### Visual hierarchy

Create hierarchy in this order:

1. typography and content order;
2. spacing and alignment;
3. tonal surface change;
4. border;
5. shadow only for overlays and true elevation.

Do not wrap every section in an outlined card. Related content should usually be grouped by proximity and section headings.

### Dashboard analytics workspace

The dashboard is a focused dark analytics workspace even when the operational routes use the light theme. This deliberate route-scoped presentation uses near-black navy surfaces, cobalt as its single interactive accent, restrained violet data-series colors, and compact panels that support fast scanning.

- The header greets the signed-in user and pairs the active period with a real navigation action.
- Four KPI cards use only current API values; growth percentages, revenue history, and comparisons must not be shown until the backend supplies measured time-series data.
- The primary chart visualizes the current pipeline value by stage. It must not be labeled as revenue over time.
- The lower panels show real deal-count distribution and the activity-type mix from the loaded page.
- The insight rail contains open tasks ordered by due date and recent activity.
- Long currency values use tabular numbers and must remain fully visible at the supported desktop widths.
- At tablet widths the insight rail moves below the primary analytics; at mobile widths every region becomes a single column and wide charts gain a deliberate horizontal viewport.
- Route-scoped shell colors must preserve WCAG 2.2 AA contrast for search chrome, account controls, navigation, and all chart labels.

### Leads operations workspace

The leads route shares the dashboard's dark navy shell but prioritizes rapid qualification over analytics decoration.

- The header contains the page identity, loaded-record count, server search, and one primary create action.
- Stage summaries use counts and proportions from the currently loaded server result only. They are interactive stage filters, never fabricated trend cards.
- List, Kanban, and Gantt remain adjacent modes of the same dataset; changing a mode must not replace working stage, conversion, assignment, or details actions.
- The desktop list uses a dense semantic table with lead identity, editable stage, source, contact, creation date, next meaningful date, and row actions. Columns without API data are not rendered as placeholders for hypothetical features.
- The filter rail exposes only server-backed criteria. Status and stage changes are applied through the collection API and remain usable with large tenant datasets.
- At tablet width the filter rail moves below the table. At mobile width stage cards scroll horizontally and the table intentionally reduces to lead, stage, and details navigation; the document itself must never gain horizontal overflow.
- Search inputs expose a single clear affordance, selects use the shared inset arrow treatment, and all stage changes retain optimistic progress and error handling.
- English and Russian labels share the generated localization catalog; user-defined stage names continue through the workspace translation center.

## 3. Design tokens

All values must be exposed through semantic CSS variables. Feature styles must not use raw brand colors.

### Color roles

Light theme:

| Token                    | Value     | Use                                    |
| ------------------------ | --------- | -------------------------------------- |
| `--color-canvas`         | `#f4f6f3` | Application background                 |
| `--color-surface`        | `#ffffff` | Primary work surface                   |
| `--color-surface-subtle` | `#edf1ed` | Side navigation, grouped controls      |
| `--color-surface-hover`  | `#e7ece8` | Hover and passive selection            |
| `--color-text`           | `#17211e` | Primary text                           |
| `--color-text-muted`     | `#5f6d67` | Secondary text                         |
| `--color-border`         | `#d8dfda` | Dividers and control borders           |
| `--color-anchor`         | `#174b40` | Primary action and selected navigation |
| `--color-anchor-hover`   | `#0f3d34` | Primary action hover                   |
| `--color-signal`         | `#9bea69` | Attention accent with dark text        |
| `--color-signal-soft`    | `#e7fbd9` | Selected and positive tonal surface    |
| `--color-danger`         | `#b4232f` | Destructive actions and errors         |
| `--color-warning`        | `#9a5b00` | Risk and overdue state                 |
| `--color-success`        | `#197448` | Confirmed positive state               |
| `--color-info`           | `#2866b1` | Neutral information                    |

Dark theme:

| Token                    | Value     | Use                                 |
| ------------------------ | --------- | ----------------------------------- |
| `--color-canvas`         | `#0e1512` | Application background              |
| `--color-surface`        | `#161f1b` | Primary work surface                |
| `--color-surface-subtle` | `#1d2924` | Side navigation, grouped controls   |
| `--color-surface-hover`  | `#26342e` | Hover and passive selection         |
| `--color-text`           | `#f2f6f3` | Primary text                        |
| `--color-text-muted`     | `#a9b6b0` | Secondary text                      |
| `--color-border`         | `#2d3b35` | Dividers and control borders        |
| `--color-anchor`         | `#9bea69` | Primary action with dark text       |
| `--color-anchor-hover`   | `#b0f284` | Primary action hover                |
| `--color-signal`         | `#9bea69` | Attention accent                    |
| `--color-signal-soft`    | `#213b2a` | Selected and positive tonal surface |

Semantic colors must be tested in context. Normal text must reach 4.5:1 contrast; large text, icons, focus indicators, and meaningful control boundaries must reach at least 3:1.

### Typography

No remote fonts. Use:

```css
font-family:
  ui-sans-serif,
  system-ui,
  -apple-system,
  BlinkMacSystemFont,
  "Segoe UI",
  sans-serif;
```

| Token         | Size / line-height | Weight | Use                                  |
| ------------- | ------------------ | ------ | ------------------------------------ |
| Display       | `32px / 38px`      | 650    | Dashboard or major workspace heading |
| Page title    | `28px / 34px`      | 650    | Route title                          |
| Section title | `18px / 24px`      | 650    | Major section                        |
| Card title    | `15px / 20px`      | 600    | Card or panel heading                |
| Body          | `14px / 20px`      | 400    | Default UI text                      |
| Body strong   | `14px / 20px`      | 600    | Names and key values                 |
| Meta          | `12px / 16px`      | 400    | Dates, counts, supporting text       |
| Control       | `14px / 18px`      | 600    | Buttons and selected tabs            |

Use sentence case. Avoid all-caps navigation and excessive bold text. Russian text must remain readable without reducing the font size.

### Spacing and sizing

Base unit: 4px.

`--space-1: 4px`, `--space-2: 8px`, `--space-3: 12px`, `--space-4: 16px`, `--space-5: 20px`, `--space-6: 24px`, `--space-8: 32px`, `--space-10: 40px`.

- Comfortable control height: 40px.
- Compact control height: 36px.
- Touch control height: at least 44px.
- Icon-only target: at least 36px desktop and 44px touch.
- Input inline padding: 12px; reserve 36px for suffix icons.
- Page padding: 24px desktop, 20px tablet, 16px mobile.
- Data row: 44px compact, 52px comfortable.

### Shape and depth

| Token              | Value   | Use                                    |
| ------------------ | ------- | -------------------------------------- |
| `--radius-control` | `10px`  | Buttons, inputs, selects               |
| `--radius-panel`   | `14px`  | Cards and drawers                      |
| `--radius-overlay` | `16px`  | Dialogs and floating menus             |
| `--radius-pill`    | `999px` | Tags, badges, segmented selection only |

Circles are reserved for avatars and icon-only controls. Standard buttons, selects, inputs, cards, and containers must not become pills by accident.

Shadows are reserved for menus, dialogs, drawers, dragged cards, and toasts. Static page sections use tonal contrast or a hairline border.

## 4. Application shell

### Desktop

- Left navigation is a stable 224px rail, collapsible to 68px.
- Brand and workspace switcher occupy the top of the navigation rail.
- Modules are grouped as Work, Communication, Insights, and Administration.
- The active route uses an anchor-colored leading marker, icon, label weight, and tonal background; color alone is insufficient.
- A 56px command bar sits above the content area and contains global search, quick create, notifications, help, and the account menu.
- The main canvas is not constrained to a marketing-site max width. Data workspaces use the available width.
- The communication dock is a resizable structural pane, 380-480px wide, not a floating card over business content.

### Tablet and mobile

- The navigation becomes an off-canvas drawer.
- The command bar retains search, quick create, and account access.
- Secondary page actions move into an overflow menu.
- Record layouts collapse from three regions to tabs and stacked sections.
- Tables become a deliberately designed mobile list or horizontal data viewport; never squeeze desktop columns until labels overlap.
- The messenger becomes a full-height sheet or route and must account for the virtual keyboard.

## 5. Page anatomy

### Index workspace

The canonical order is:

1. page header: title, result count, primary create action;
2. view strip: saved view and semantic view switcher;
3. filter toolbar: search, quick filters, filter builder, columns, density, export;
4. contextual bulk bar when records are selected;
5. content surface: table, board, calendar, or timeline;
6. preview drawer for the selected record.

Toolbars must use stable zones and must not form broken multi-line rows. At narrow widths, low-priority actions move to overflow before the toolbar wraps.

### Semantic view matrix

| Entity     | Default         | Additional views                                 | Explicit exclusion                        |
| ---------- | --------------- | ------------------------------------------------ | ----------------------------------------- |
| Contacts   | Table + preview | Saved filtered lists                             | Kanban and Gantt                          |
| Companies  | Table + preview | Saved filtered lists                             | Kanban and Gantt                          |
| Leads      | Table + preview | Stage board                                      | Decorative Gantt without meaningful dates |
| Deals      | Pipeline board  | Table, forecast timeline                         | Project-style dependency Gantt            |
| Projects   | List            | Board, calendar, dependency Gantt                | Sales pipeline semantics                  |
| Tasks      | My work list    | Board, calendar, dependency Gantt when scheduled | Revenue forecast semantics                |
| Activities | Agenda          | Calendar, timeline                               | Generic Kanban                            |

View, filters, sorting, saved view, density, and selected record must be reflected in URL or persisted view state so switching views does not reset the user's work.

#### Companies workspace

The companies index uses the dark, high-density workspace language: a compact title/count/search/create header, four factual loaded-page summary cards, a filter and saved-view strip, a responsive semantic table, and an adjacent quick-view panel. Summary cards may show only values returned by or derived from the loaded API page; deal revenue, contact counts, growth, and activity claims are not displayed until dedicated aggregate endpoints exist. Desktop keeps the quick view adjacent to the table, tablet stacks it below the data, and mobile collapses secondary columns into the full record rather than shrinking text below a usable size. Search has one native clear mechanism, primary button icon/text alignment is invariant across breakpoints, and all existing create, trash/restore, cursor loading, and saved-view behavior remains functional.

#### Tasks workspace

The task workspace presents tasks as the primary view while retaining calls, meetings, notes, and the combined activity feed as explicit tabs. Its summary cards are derived only from the loaded activity response: loaded tasks, open, completed, overdue, high priority, and due today. The main table preserves completion and assignment actions; the adjacent insight rail shows an actual current-month due-date calendar, priority distribution, and overdue items. No project title or assignee display name is inferred from opaque IDs. Desktop uses a table plus insight rail, tablet places three insight cards below the table, and mobile keeps only task identity and status in the row while exposing the complete record actions through the normal flow.

#### Calendar workspace

The calendar uses a dense Monday-first month grid with day and week alternatives, type-coded event cards, and a current-period counter. The adjacent rail reuses the loaded period data for a navigable mini-calendar, the next five real events, and visibility-scope filters; it never invents participants or meetings. Selecting a day opens that date for creation or focused day review. At tablet widths the rail becomes a set of cards below the calendar; on mobile the month grid remains horizontally scrollable so dates and event labels keep a usable size instead of collapsing into unreadable cells.

### Preview drawer

- Opens without losing list position, filters, or selection.
- Shows identity, stage/status, owner, next action, essential properties, associations, and recent activity.
- Supports short inline edits and quick actions.
- Links to the full record for complex work.
- Returns keyboard focus to the triggering row/card when closed.

### Full record

Header:

- a shared ghost **Back** control with chevron and label;
- record avatar/mark, title, company or parent context;
- stage/status control;
- responsible owner;
- primary action and overflow menu.

The navigation control may render as an anchor for correct routing semantics, but it must use the same visible button component, target size, focus state, and pressed state as other controls.

Main layout:

- summary rail: key properties, tags, custom fields;
- central workspace: Overview, Activity, Messages, Tasks, Files, Audit;
- context rail: related records, responsible users, watchers, next activity, attachments.

Structured business activity, human discussion, and immutable audit history must remain separate.

## 6. Components

### Buttons

Supported variants: primary, secondary, ghost, danger, icon.

- Icon and label use an 8px gap and share a visual baseline.
- Primary actions use the anchor color; lime is a signal accent, not a default fill for every action.
- Every button has default, hover, focus-visible, active, disabled, and loading states.
- Active feedback may use `transform: scale(0.97)` for 100-160ms.
- Never use `transition: all`.
- Navigation actions use anchors styled through the same component contract; mutations use buttons.

### Inputs, selects, and comboboxes

- Use one shared field component and Angular Material public theming APIs.
- Do not mix native selects and Material fields in the same workflow.
- Labels remain visible; placeholders are examples, not labels.
- Select arrows and suffix actions reserve their own inset area and never touch the edge.
- Focus uses a visible 2px outline/ring with sufficient contrast.
- Error copy appears below the field, is linked with `aria-describedby`, and is not communicated by color alone.
- Multi-user and department selection uses a searchable combobox, not an over-wide native select.

### Segmented controls

- Use only for two to four mutually exclusive view modes or filters.
- The group has a subtle tonal surface; the selected item has a solid surface and stronger text.
- The outer group and selected item share the same radius family; do not place a circular pill inside a square container.
- Implement roving keyboard focus or use an accessible Material button-toggle group.

### Tables

- Header remains visually quiet but clearly separated.
- Rows use hover and selection surfaces rather than heavy borders.
- Names and next actions receive priority; metadata is muted.
- Support keyboard navigation, cursor pagination, sorting, filtering, column state, selection, and preview.
- Bulk selection replaces the default toolbar with a contextual action bar.
- Loading uses stable skeleton rows; empty and error states preserve the table frame.

### Kanban

Each card shows only title, amount or priority, company/contact, responsible avatar, next activity, overdue/risk state, and age in stage where useful.

- Columns show count and relevant aggregate.
- Load cards per stage with a bounded page size.
- Drag has pending, confirmed, conflict, and rollback states.
- Every drag action has a keyboard/menu alternative.
- Stage changes that require fields open a focused completion dialog.

### Assignment editor

Do not place role, subject, primary checkbox, and Add in a single horizontal form.

Canonical flow:

1. Existing participants are a compact list with avatar, name, department, role, primary marker, and overflow action.
2. A visible `Add participant` button opens an anchored popover on desktop and a dialog/sheet on mobile.
3. The form contains a searchable user/department combobox, role choice, and a clearly styled `Primary responsible` switch when applicable.
4. Save is explicit. Errors stay inside the overlay. On success, focus returns to `Add participant`.

### Custom fields

- Administrator-defined fields must render with the same field primitives as system fields.
- Create forms show required custom fields and place optional custom fields under `More details`.
- Full record pages group custom fields into named sections and support permission-aware inline editing.
- Unsupported, invalid, or stale definitions show a recoverable state; they never silently disappear.

## 7. Messenger and record discussions

The messenger must feel like a complete communication workspace, not a form inside a drawer.

### Layout

Full route on desktop:

- conversation list: 280-320px;
- message pane: flexible, minimum 480px;
- optional details pane: 280-340px.

Dock:

- 400-460px wide;
- conversation list and active conversation are separate states;
- composer remains pinned to the bottom;
- pinned-message strip remains below the conversation header;
- no business controls are obscured when the dock opens.

### Message anatomy

- Group consecutive messages from the same sender.
- Show avatar and sender once per group, with compact timestamps.
- Own messages use a soft signal tint; other messages use the surface color.
- Long text, links, file names, and Russian copy wrap safely.
- Hover-only actions also have keyboard focus and a mobile long-press/menu equivalent.

### Message lifecycle

Own messages expose a real state machine:

`sending -> accepted by server -> delivered to recipients -> read -> failed`

Use a clock, single check, double check, read avatars/label, and error-retry icon from the shared SVG icon set. Do not display a confirmed state before server acknowledgement. Group chat exposes `Read by` without leaking participants who cannot access the conversation.

### Media

Render by validated media type:

- image: bounded preview, dimensions reserved before load, open/download action;
- video: poster or neutral placeholder, duration, native-accessible controls, explicit loading and failure states;
- voice: play/pause, duration, seek track or waveform, elapsed time, playback rate, download action;
- file: type icon, display name, size, download state;
- upload: progress, cancel, retry, and failure reason.

Media must load on demand and remain playable after reload. Object URLs are revoked when no longer needed.

### Message actions

Use consistent SVG icons for like, reaction picker, reply, favorite, pin, forward, edit, copy link, and delete. A shared pin is different from a private favorite.

- Pinning updates both the message action and the pinned-message area atomically.
- Favorite is private to the current user.
- Reactions show the chosen reaction and count, while the action itself uses the icon set rather than a raw emoji button.
- Action buttons have accessible names and tooltips.

### Calls

Audio and video call controls are visible only when a provider is configured and the user is authorized. The UI has explicit requesting-permission, connecting, connected, reconnecting, ended, declined, and failed states. A failed call must not leave camera or microphone tracks active.

## 8. Feedback and motion

### Deal workspace

The deals route is a dense, dark pipeline workspace rather than a collection of disconnected cards. Its first row keeps the page identity, Kanban/List/Gantt tabs, server-backed search, and the primary create action on one visual axis. A second row contains the active pipeline, four metrics calculated only from records actually loaded by the bounded data source, and the status filter.

Kanban columns use restrained stage tinting, compact monetary cards, keyboard-accessible stage selectors, and CDK drag-and-drop with rollback. Selecting a card opens an adjacent quick-view pane; the canonical details route remains the editing surface. On narrower screens the pane moves below the board and the board scrolls horizontally without expanding the document viewport. List and Gantt are alternate views of the same server state, not separate datasets.

Never total unlike currencies into a fictional number. When a loaded result contains multiple currencies, show the localized mixed-currency label and preserve the individual amounts. Loaded counts and conversion values must be labeled as current-view summaries rather than workspace-wide analytics.

### Contact workspace

The contacts route uses the same dark, high-density workspace grammar while remaining a server-driven data surface. Page identity, quick search, import/export, and the primary create action form the first row. A restrained summary strip reports only the currently loaded contact page: loaded, active, linked to a company, and carrying an email address. It must never imply workspace totals when the cursor API does not return them.

AG Grid Community remains the accessible list engine and stays lazy with the route. The main table exposes contact, company, phone, email, owner, status, and creation date; selection continues into the existing bulk-action surface. The right rail contains only working server-backed search/status filters and saved views. On compact layouts the filter rail moves below the list rather than compressing the table into unusable columns, while horizontal table overflow remains contained within the grid.

- Short successful operations may use toasts.
- Critical failures, validation, permissions, conflicts, and unsaved state stay near the affected content.
- Optimistic updates require rollback and a visible conflict path.
- Loading indicators must not shift the surrounding layout.

Motion is functional and infrequent:

- button press: 100-160ms;
- tooltip or small popover: 125-180ms;
- select/menu: 150-200ms;
- drawer/dialog: 180-240ms;
- easing: `cubic-bezier(0.23, 1, 0.32, 1)` for entry and response;
- animate only transform and opacity when practical;
- no animation for repeated keyboard commands;
- reduced-motion removes positional movement while preserving helpful fades.

## 9. Accessibility

Meet WCAG 2.2 AA.

- All workflows are operable by keyboard.
- Focus is visible, not hidden behind sticky UI, and restored logically after overlays close.
- Dialogs trap focus and expose a labeled close action.
- Combobox, grid, tabs, menus, and dialogs follow WAI-ARIA Authoring Practices.
- Pointer targets are at least 24x24px with spacing; important and touch controls target 44x44px.
- Drag operations have single-pointer and keyboard alternatives.
- Status is never communicated only by color, shape, or position.
- At 200% zoom, core workflows remain usable; at 400%, content reflows without losing actions.
- Forced-colors and high-contrast states retain boundaries, focus, and selection.

## 10. Localization

- Every visible label, tooltip, accessible name, validation message, status, empty state, notification, and server-error mapping uses a translation key.
- English is the source locale; Russian completeness is enforced.
- Components must tolerate at least 35% text expansion.
- Avoid fixed widths for labels and action groups.
- Dates, numbers, currency, plural forms, locale, and timezone use native locale-aware formatting.
- Truncation is allowed only when the complete value remains available through an accessible name, tooltip, or details view.

## 11. Implementation boundaries

- Angular Material/CDK/Aria remain the behavioral foundation.
- Use Material public APIs, theme variables, host classes, and documented styling hooks. Never depend on private `.mdc-*` DOM structure.
- Use the existing local SVG icon component and extend it with a coherent open icon set; do not introduce icon fonts.
- Feature pages consume shared primitives instead of duplicating form, toolbar, card, or message CSS.
- AG Grid remains lazy and Community-only on complex list routes.
- Messenger, media, calls, reports, and AG Grid stay outside the initial bundle.
- No external font or image request is required for the application shell.

## 12. Quality gate

A component is incomplete until relevant states are verified:

- default, hover, focus-visible, active, selected;
- disabled and permission denied;
- loading, empty, success, warning, error, conflict;
- compact and comfortable density;
- light, dark, forced colors, reduced motion;
- English and Russian at realistic lengths;
- desktop, tablet, and mobile;
- keyboard and screen-reader semantics.

Visual acceptance uses real application data and the existing key viewports: 1440x900, 1024x768, and 390x844. Screenshots are evidence, not the source of truth; component and interaction tests remain mandatory.
