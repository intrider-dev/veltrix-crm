# VeltrixCRM interface redesign prompt

Use this prompt for the implementation phase. `DESIGN.md` remains the normative design contract.

---
Redesign the existing VeltrixCRM Angular SPA into the **Veltrix Signal** interface described in `DESIGN.md`. This is a complete visual and interaction-system replacement, not a new static mockup and not a page-by-page CSS patch.

## Required preparation

1. Read `DESIGN.md`, `docs/DESIGN_RESEARCH.md`, `docs/STATE.md`, ADR 0017, and the current shared styles/components.
2. Inventory every control, toolbar, card, table, drawer, record page, empty state, and messenger state currently implemented.
3. Capture baseline screenshots at 1440x900, 1024x768, and 390x844 for the dashboard, contacts, companies, leads, deals, lead details, deal details, projects, reports, settings, and messenger.
4. Record current accessibility and bundle results before replacing components.

## Design direction

Create an original light-first sales workspace using a warm neutral canvas, white work surfaces, deep evergreen anchor, vivid lime signal accent, strong typography, restrained borders, and compact contextual tools. The public FluxCRM dashboard concept is a mood reference only. Do not copy its logo, assets, exact palette, decorative cut-outs, diagonal patterns, or page layouts.

The product must feel materially different from the current blue-gray admin interface. Hierarchy comes from typography, spacing, and tonal surfaces before borders or shadows. Do not wrap every section in a card.

## Shared system first

Implement semantic tokens and reusable primitives before redesigning feature pages:

- application shell and navigation;
- button and icon-button;
- field, select, searchable combobox, checkbox, radio, and switch;
- segmented view control and tabs;
- page header, view strip, filter toolbar, and contextual bulk bar;
- panel, section, drawer, dialog, popover, menu, tooltip, toast, and error panel;
- table shell, row actions, skeleton, empty state, and pagination;
- Kanban column/card and stage move feedback;
- record header, summary section, property section, and preview drawer;
- participant list and Add participant overlay;
- message bubble, composer, media players, message actions, pinned strip, and delivery/read indicators.

Use Angular Material/CDK/Aria as the behavioral foundation. Customize through public APIs, system theme variables, host classes, and documented styling hooks. Do not style private `.mdc-*` descendants. Do not add another universal UI kit, icon font, external font, or enterprise grid package.

## Information architecture

Use `workspace -> saved view -> list/board -> preview drawer -> full record` as the primary model.

- Contacts and companies: table plus preview drawer.
- Leads: table and stage board plus preview drawer.
- Deals: pipeline board, table, and forecast timeline.
- Projects and scheduled tasks: list, board, calendar, and dependency-aware Gantt.
- Activity: agenda, calendar, and timeline.

Switching views must preserve filters, saved view, sorting, density, and selection. Do not add view modes that do not match the entity's semantics.

## Record pages

Replace inline-looking back links with the shared ghost button appearance while retaining correct link semantics. The header shows identity, stage/status, responsible owner, primary action, and overflow.

Organize full records into:

- summary properties and custom fields;
- Overview;
- Activity;
- Messages;
- Tasks;
- Files;
- Audit;
- related records, assignments, watchers, and next activity.

Administrator-defined fields must look and behave like built-in fields. Required custom fields appear in create flows; optional fields appear under progressive disclosure.

## Assignment editor

Remove the horizontal grid of role select, user/department select, primary checkbox, and Add button. Show existing participants as a compact list. A styled `Add participant` button opens a searchable, keyboard-accessible popover/dialog with subject, role, and primary responsibility. Keep errors inside the overlay and restore focus after close.

## Messenger

Replace the current floating form-like chat with a real communication workspace:

- resizable dock on desktop and full-height sheet/route on small screens;
- separate conversation list and active conversation states;
- grouped message bubbles, pinned-message strip, sticky composer;
- coherent SVG icons for like, reaction picker, reply, favorite, pin, forward, edit, copy, retry, and delete;
- distinct shared pin and private favorite;
- real message lifecycle: sending, accepted by server, delivered, read, failed;
- `Read by` for authorized group participants;
- image preview, video player, voice player with play/pause/seek/duration/rate, and file card;
- upload progress, cancel, retry, media loading, media failure, and playback failure;
- draft preservation and retry without duplicate messages;
- visible call states and immediate media-track cleanup on failure/end.

Do not use raw emoji as the like, favorite, or pin action button. Reactions may display their chosen content in a reaction chip, but the action opens through the shared icon set.

## Interaction and accessibility

Meet WCAG 2.2 AA. Support keyboard operation, visible/unobscured focus, focus restoration, dialog trapping, drag alternatives, high contrast, reduced motion, 200% zoom, localized accessible names, and minimum pointer targets.

Use only purposeful motion. Buttons get subtle 100-160ms press feedback. Popovers and menus use 125-200ms ease-out; drawers/dialogs use 180-240ms. Repeated keyboard actions are instant. Avoid `transition: all`, layout-property animation, scale-from-zero, and decorative stagger in operational lists.

## Localization and responsive requirements

Every visible string, tooltip, accessible name, validation message, empty/error state, and server error uses the localization system. Verify English and Russian with realistic content and at least 35% text expansion.

Verify desktop, tablet, and mobile as distinct compositions. No toolbar may overlap or create a broken control row. The mobile keyboard must not cover the messenger composer.

## Delivery order

1. Tokens and theme foundation.
2. Shared controls and overlays.
3. Shell and navigation.
4. List workspace and preview drawer.
5. Record pages and assignment flow.
6. Leads/deals views.
7. Messenger/media/read-state experience.
8. Remaining features and settings.
9. Dark/high-contrast/reduced-motion/mobile passes.
10. Screenshot, accessibility, test, bundle, and regression verification.

After each stage, run formatter, lint, typecheck, focused component tests, localization checks, and relevant browser flows. Do not claim a view or media state is complete until it has been exercised against the running application.

## Acceptance criteria

- No page-specific copy of shared control styles.
- No private Material DOM selectors.
- No raw emoji for like/favorite/pin controls.
- No native/Material field mixture in one workflow.
- No overlapping or edge-clipped toolbar, assignment, messenger, select, or form layout at the target viewports.
- Record navigation has button-level visual affordance and accessible semantics.
- Required and optional custom fields appear in the correct record flows.
- Voice and video render through the correct playable component after reload.
- Pin state and pinned strip stay synchronized.
- Message accepted/delivered/read/failure states are visible and backed by real server state.
- List/board/timeline switches preserve the user's working context.
- English and Russian pass completeness and layout checks.
- Keyboard, axe smoke, dark theme, reduced motion, console, and visual smoke tests pass on the running production-like application.
- Bundle and performance changes are measured and documented; no result is invented.

---
