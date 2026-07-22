# ADR 0015: Stage access is an explicit role overlay

- Status: accepted
- Date: 2026-07-22

## Context

Resource-level lead/deal permissions are insufficient when a workspace must hide sensitive funnel stages or prevent particular roles from entering or leaving them. Encoding this policy only in the Angular client would still expose records through list, detail, search, export, and direct API calls.

## Decision

Lead stages and deal pipeline stages may have bounded role rules for `view`, `enter`, and `leave`. A stage with no rules inherits the existing resource-level permission. Once at least one explicit rule exists, an active membership without a matching role row is denied. Workspace owners and administrators retain a deliberate bypass so the workspace cannot lock itself out.

The application checks transitions before mutation and PostgreSQL helper functions filter stage lists, record lists, detail reads, Kanban pages, and global-search candidates. Rules are replaced atomically under a workspace advisory/configuration lock, use composite tenant foreign keys and forced RLS, and create an audit/outbox event. The editor reuses a localized Material-based component on both lead-stage and pipeline settings routes.

## Consequences

- Hidden-stage records are not merely hidden by the browser; ordinary read surfaces enforce the same role overlay.
- Transition authorization distinguishes leaving the source from entering the target.
- The rule model stays small and explainable instead of becoming a general policy language.
- Workspace summary tables currently aggregate tenant totals, not role-specific stage visibility. Role-filtered dashboards/reports remain an explicit hardening item and are not claimed complete.

