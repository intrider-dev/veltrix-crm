# ADR 0006: Optional, consent-gated AI provider boundary

- Status: Accepted
- Date: 2026-07-21

## Context

Timeline summaries, follow-up drafts, suggested next actions, and duplicate suggestions can assist a sales user, but CRM inputs often contain PII and commercially sensitive content. An AI service must not become required infrastructure, silently transmit tenant data, or worsen the base idle profile.

## Decision

- Define a small provider interface for the four explicit assistance use cases.
- Support Ollama-compatible local and documented OpenAI-compatible HTTP adapters through configuration, without linking either provider to core CRM availability.
- Keep the feature disabled by default and hide its UI when no provider is configured.
- Require explicit deployment configuration and an in-product confirmation before sending PII to an external provider. The confirmation must identify the configured provider class and data scope.
- Send the minimum bounded context, use tenant/user rate limits, request timeout and cancellation, and record a safe audit event without prompts, provider keys, or raw responses.
- Treat model output as untrusted draft text: encode on output, never execute it, never mutate records without a normal authorized confirmation, and never use it for authorization.
- Keep credentials server-side and redacted; do not expose provider keys to the SPA.

## Consequences

- The base app works, benchmarks, and idles with AI disabled and no AI container.
- Operators are responsible for provider terms, retention, region, model licenses, and data-processing agreements.
- Local providers reduce external transfer but still require host isolation and model security.
- Prompt injection, hallucination, and sensitive-data inference remain residual risks; every output is advisory.
- Provider-specific reliability/cost is not a core SLA and must be measured separately.

## Alternatives rejected

- **Mandatory AI service:** violates the minimal base profile and availability boundary.
- **Browser-to-provider calls:** exposes credentials/data and bypasses audit/rate/consent controls.
- **Implicit consent in global terms:** insufficient for context-dependent external PII transmission.
- **Autonomous record updates:** model output is untrusted and requires normal validation/authorization/user confirmation.
- **Store full prompts/responses for debugging:** creates a new sensitive-data repository without a justified retention need.
