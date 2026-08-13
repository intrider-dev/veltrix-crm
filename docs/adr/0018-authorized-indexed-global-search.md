# ADR 0018: Indexed global search uses an explicitly authorized security boundary

- Status: accepted
- Date: 2026-07-22

## Context

PostgreSQL full-text and trigram indexes keep global CRM search small and fast, but forced row-level security can prevent useful index plans. A broad `SECURITY DEFINER` function owned by a superuser would recover the plan at the cost of turning search into a tenant and object-authorization bypass.

## Decision

The indexed search function is owned by a dedicated `NOLOGIN` role with only the exact schema, function, and read grants it requires. The function accepts the authenticated actor and workspace context, first proves active membership, and then applies entity permission, stage-access, activity-audience, and note-visibility predicates before ranking or returning a document. Nullable display fields are normalized at the SQL boundary. The ordinary request role cannot impersonate the search owner.

Application guards remain mandatory; the function is a narrow, reviewed authorization boundary rather than a general RLS bypass. Negative integration tests exercise contacts, leads, deals, restricted stages, private notes, custom roles, and nullable search metadata.

Migration `000035` keeps the same authorization boundary but computes actor membership, system-role, and resource-permission facts once per request. It bypasses per-stage checks only for the already-proven system-admin path and bounds the pre-ranked candidate set in proportion to the requested result limit. This avoids repeated permission joins without turning ranking or pagination into an authorization shortcut.

## Consequences

- PostgreSQL can use the search indexes without granting the web process a superuser or `BYPASSRLS` connection.
- Every newly searchable entity requires an explicit authorization branch and negative test.
- Search permissions cannot be inferred only from the presence of a `search_documents` row.
