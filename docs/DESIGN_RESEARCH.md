# CRM product and interface research

Date: 2026-07-31

## Scope and source policy

It is not possible to exhaustively review every public page about CRM systems. This review uses a deliberate corpus of current primary sources: official product documentation, official design systems, W3C standards, and the public portfolio page named by the product owner. Search-result aggregators and generic “best CRM” articles were excluded.

The FluxCRM portfolio concept is used only as a visual reference. Functional requirements come from official CRM documentation and standards, not from portfolio mockups or unverified marketing claims.

## Which FluxCRM reference was selected

The same dpopstudio concept is also shown in [FluxCRM — CRM Dashboard](https://dribbble.com/shots/24906667--FluxCRM-CRM-Dashboard).

Several unrelated products use the name FluxCRM. The selected reference is [FluxCRM — Customer Retention Marketing Dashboard](https://dribbble.com/shots/25226262-FluxCRM-Customer-Retention-Marketing-Dashboard) by dpopstudio because it presents a coherent set of desktop, modal, table, calendar, and mobile CRM screens. The separate [fluxcrm.app](https://fluxcrm.app/) product is documented as a distinct product and is not assumed to be the same design system.

Observed visual properties of the selected portfolio concept:

- off-white workspace canvas;
- white and pale-gray surfaces;
- deep evergreen anchor and bright lime accent;
- strong black typography and large numeric hierarchy;
- rounded rectangular controls, selective pills, circular icon actions;
- compact icon-led navigation;
- table, dashboard, modal, calendar, and mobile variants;
- sparse shadows and clear surface separation.

The concept also contains traits that should not be copied: branded assets, proprietary visual motifs, diagonal card patterns, decorative notches, exact colors, and low-density presentation layouts.

## Current interface review

The review used current source tokens/components and the existing real application screenshots in `output/playwright/`.

| Before | After | Why |
| --- | --- | --- |
| Blue-gray admin shell with many similar outlined panels | Warm neutral canvas, deep evergreen anchor, lime signal accent, and fewer structural borders | Produces a distinct product character and clearer hierarchy |
| Every section reads as a bordered card | Group related content through spacing and tonal surfaces; reserve cards for meaningful units | Reduces visual fragmentation |
| Toolbars form multiple boxed rows and wrap unpredictably | Stable header, view strip, filter toolbar, and contextual bulk bar | Makes list routes predictable and prevents cramped layouts |
| Native selects coexist with Material fields | One shared field/combobox contract using public Material APIs | Fixes arrow, padding, focus, error, and cross-page inconsistencies |
| Assignment role, subject, checkbox, and action share one horizontal grid | `Add participant` opens a focused popover/dialog | Avoids overflow and gives the task a clear beginning and completion |
| Record back navigation looks like an inline text link | Shared ghost button appearance with correct navigation semantics | Improves target size, recognition, and consistency |
| Record fields are split into page-specific forms | System and custom fields use the same section and field primitives | Makes administrator-added fields first-class and predictable |
| Chat appears as a floating form over CRM content | Resizable structural dock plus a full messenger route | Gives conversations enough space and avoids covering work |
| Media attachments have generic or ambiguous presentation | Dedicated image, video, voice, and file renderers with loading/error states | Makes type and playback behavior immediately clear |
| Pin and reactions mix raw symbols with icons | One coherent SVG action set; shared pin and private favorite are distinct | Removes visual noise and clarifies action meaning |
| Sent messages lack an end-to-end visible lifecycle | Sending, server accepted, delivered, read, and failed states | Users can tell whether a message reached the system and recipient |
| Dense pages rely on the same layout at every width | Purpose-built desktop, tablet, and mobile composition | Prevents overlap and preserves primary actions |

## Cross-product findings

### Working views

- [Salesforce Kanban](https://help.salesforce.com/s/articleView?id=sf.kanban_use.htm&language=en_US&type=5) supports search/filter, drag updates, inline edits, and a details panel while preserving the list context.
- [HubSpot board view](https://knowledge.hubspot.com/records/manage-records-in-board-view) pairs table/board modes with saved views, quick filters, aggregate values, record preview, bulk actions, and required-field completion when moving a stage.
- [Pipedrive pipeline view](https://support.pipedrive.com/en/article/pipeline-view) keeps title, contact, value, label, owner, and next activity close to the pipeline card.
- [Zoho module views](https://help.zoho.com/portal/en/kb/crm/getting-started/articles/your-first-day-as-a-crm-user) exposes different views for different work: list, Kanban, timeline, split, map, and chart.
- [Dynamics Sales guidance](https://learn.microsoft.com/en-us/dynamics365/guidance/develop/ui-ux-guidance-sales-components) treats the pipeline as a place to prioritize and act, not merely visualize.

Conclusion: views must be semantic. Contacts need scanning and preview; leads need list and stage board; deals need pipeline and forecast; projects and scheduled tasks justify calendar and dependency-aware Gantt.

### Record workspace

- [HubSpot record cards](https://knowledge.hubspot.com/records/use-cards-on-records) separate properties, activities, reports, associations, and quick actions.
- [HubSpot record customization](https://knowledge.hubspot.com/object-settings/customize-records) supports role/team-specific sections and conditional cards.
- [Pipedrive detail view](https://support.pipedrive.com/en/article/detail-view) combines stage progress, linked contacts, activities, notes, email, and files.
- [Dynamics opportunity lifecycle](https://learn.microsoft.com/en-us/dynamics365/sales/create-edit-opportunity-sales) keeps probability, expected close, stakeholders, products, and stage progress within the opportunity workflow.

Conclusion: use preview for rapid inspection and a structured full record for deeper work. Overview, Activity, Messages, Tasks, Files, and Audit must stay distinct.

### Communication

- [Microsoft Teams read receipts](https://support.microsoft.com/en-us/teams/chat/use-read-receipts-for-messages-in-microsoft-teams) distinguishes sent/delivered from seen and provides `Read by` for supported group conversations.
- [Slack message composition](https://slack.com/help/articles/201457107-Send-and-read-messages) includes files, mentions, audio/video clips, formatting, drafts, and scheduled sending.
- [Slack action semantics](https://slack.com/help/articles/360002063088-Understand-your-actions-in-Slack) distinguishes a shared pin from a private saved item.

Conclusion: message state, shared pin, private favorite, media type, and retry behavior are separate concepts and need separate data/UI states.

### Accessibility and component construction

- [WCAG 2.2](https://www.w3.org/TR/WCAG22/) requires visible and unobscured focus, alternatives to dragging, minimum pointer targets, and robust status semantics.
- [WAI-ARIA Grid](https://www.w3.org/WAI/ARIA/apg/patterns/grid/), [Combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/), and [Dialog](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/) define the keyboard and focus contracts needed by CRM data workspaces.
- [Fluent accessibility](https://fluent2.microsoft.design/accessibility) and [Fluent layout](https://fluent2.microsoft.design/layout) emphasize predictable hierarchy, focus management, contrast, and spacing as a relationship signal.
- [Angular Material theming](https://material.angular.dev/guide/theming) provides public theme-level customization; implementation must not depend on private internal DOM.

## Product principles derived from the evidence

1. **Action before decoration.** Metrics, cards, and charts must lead to a useful next action.
2. **Preserve context.** Preview drawers and view state prevent expensive back-and-forth navigation.
3. **Use the right view for the data.** List, board, timeline, calendar, and Gantt are not interchangeable skins.
4. **Keep one component grammar.** Buttons, fields, selects, tabs, toolbars, drawers, and messages share tokens and behavior.
5. **Separate business concepts.** Activity, human discussion, audit, notification, and mail are related but not identical streams.
6. **Make state visible.** Pending, accepted, delivered, read, failed, optimistic, rollback, and conflict states cannot be inferred.
7. **Make customization first-class.** Custom fields and permissions use the same interaction quality as built-in fields.
8. **Design mobile intentionally.** Responsive behavior changes composition, not merely width.

## Decision

Adopt the original **Veltrix Signal** direction described in the root `DESIGN.md`. It borrows the optimistic neutral/evergreen/lime mood from the selected FluxCRM concept, combines it with action-centered CRM information architecture, and applies the repository's accessibility, localization, performance, and component constraints.
