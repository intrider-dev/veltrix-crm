# ADR 0016: Personal mailboxes use bounded IMAP/SMTP adapters

- Status: accepted
- Date: 2026-07-22

## Context

Users need a first-class corporate-mail section without adding a required mail server, message broker, or Node.js runtime. Mail credentials are higher-risk than ordinary CRM content, and remote mail endpoints create SSRF, resource-exhaustion, HTML-tracking, and cross-user disclosure risks.

## Decision

Each active membership may configure a bounded number of personal IMAP/SMTP accounts. Credentials are encrypted with AES-256-GCM using the deployment keyring and account/workspace/user-bound associated data; the API never returns a password. Forced RLS scopes every mailbox table to both the current workspace and current actor, including owner/admin sessions.

The adapter allows only TLS or STARTTLS on an explicit IMAP/SMTP port set, resolves and validates every endpoint before connecting, rejects loopback/link-local/multicast/unspecified addresses, and rejects private networks unless the operator explicitly enables the corporate-network policy. Connections, folders, messages, recipients, MIME parts, body sizes, timeouts, and cached pages are bounded. The browser renders only extracted plain text and never remote HTML.

The Angular `/mail` route is lazy, uses the existing Material component layer, and provides account setup, manual sync, folder/message navigation, plain-text reading, and composition. Creating an outbound message persists its stable Message-ID, HTTP idempotency result, audit intent, and a unique `platform.jobs` delivery record in one tenant transaction. A leased worker commits `sending` before SMTP I/O, performs SMTP without holding a database transaction, and persists `sent`, retryable `failed`, or terminal `dead` state in a new short transaction.

Only failures known to occur before SMTP submission are retried automatically. A process/commit failure after the durable `sending` transition, or an ambiguous SMTP DATA result, becomes `mail_delivery_uncertain` and is never submitted automatically again. This at-most-once boundary prefers an explicit uncertain state over duplicate customer email; a stable Message-ID is reused on every safe retry.

Manual IMAP sync is staged similarly: a short transaction validates the actor, decrypts a short-lived credential snapshot, and commits `syncing`; bounded remote reads happen after commit; a new transaction applies the bounded snapshot or persists a stable error state. Concurrent sync of one account fails closed while it is already `syncing`.

## Consequences

- The base runtime remains one application container plus PostgreSQL; mail servers stay external.
- Personal mail remains private even from another workspace administrator through the application database role.
- Basic/app-password authentication is implemented; provider OAuth2 is not yet implemented.
- SMTP delivery is durable and retryable without holding the request transaction. Exact-once external SMTP delivery is impossible without provider idempotency; ambiguous attempts are intentionally terminal and require a future explicit operator/user reconciliation flow.
- Manual sync no longer holds a database connection during remote folder/message retrieval. Scheduled sync remains roadmap work, and on-demand single-message body retrieval still uses the older bounded request transaction path.
