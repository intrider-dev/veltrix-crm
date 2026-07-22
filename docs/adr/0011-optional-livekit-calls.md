# ADR 0011: LiveKit as an optional calls provider

- Status: accepted
- Date: 2026-07-22

## Context

VeltrixCRM needs real audio/video calls inside authorized conversations. Implementing a custom WebRTC signalling, selective-forwarding, NAT traversal, and media security stack would create disproportionate correctness and operational risk. Calls must not add Redis or another mandatory process to the two-container base profile.

## Decision

Use LiveKit behind a small `calls.Provider` interface. Calls are disabled by default. The optional `calls` Compose profile starts a pinned single-node LiveKit server for local development; production operators configure an external or self-hosted WSS endpoint.

The Go application signs two-to-ten-minute HS256 participant tokens itself with a compact JWT dependency. Grants contain only room join, publish, and subscribe permissions for one server-generated room and one authenticated user identity. Provider secrets never reach the browser. Call invitations and lifecycle signals use recipient-scoped SSE and are never published to workspace-wide outbox consumers.

The browser client is imported dynamically from the chat feature, so disabled calls do not add LiveKit to the initial bundle. CSP `connect-src` and camera/microphone Permissions Policy are relaxed only when the provider is enabled, using the validated exact configured origin.

## Consequences

- The base runtime remains the application plus PostgreSQL.
- Single-node local LiveKit needs no Redis; multi-node LiveKit deployments may need provider-specific infrastructure outside the base CRM profile.
- Production calls require trusted TLS/WSS and deliberate TCP/UDP/TURN/network configuration.
- CRM authorization controls call membership; LiveKit handles real-time media transport.
- Call metadata remains tenant-scoped in PostgreSQL, while media is transient in the configured provider.
