# Dependency inventory and policy

Last reviewed: 2026-07-21. Versions are exact direct versions from `package.json`, `apps/web/package.json`, `apps/api/go.mod`, `Dockerfile`, and `compose.yaml`; transitive versions are pinned by `pnpm-lock.yaml` and `apps/api/go.sum`.

Licenses below describe the upstream projects at the recorded versions and must be confirmed by the dependency-license CI report before a release. This document is not legal advice.

## Production frontend dependencies

Frontend dependencies are compiled into static assets; Node.js is not present in the production runtime image.

| Dependency                     |   Version | Purpose                                                   | License     |
| ------------------------------ | --------: | --------------------------------------------------------- | ----------- |
| `@angular/core`                |    22.0.7 | Standalone components, zoneless runtime, signals, DI      | MIT         |
| `@angular/common`              |    22.0.7 | HTTP/common directives and platform utilities             | MIT         |
| `@angular/compiler`            |    22.0.7 | Angular template compilation/runtime metadata             | MIT         |
| `@angular/forms`               |    22.0.7 | Typed/reactive and signal-form integration                | MIT         |
| `@angular/platform-browser`    |    22.0.7 | Browser bootstrap/rendering                               | MIT         |
| `@angular/router`              |    22.0.7 | Route guards and lazy feature boundaries                  | MIT         |
| `@angular/service-worker`      |    22.0.7 | PWA app shell/update support                              | MIT         |
| `@angular/material`            |    22.0.5 | Material 3 UI components/tokens                           | MIT         |
| `@angular/cdk`                 |    22.0.5 | Overlay, drag/drop, a11y, virtualization behavior         | MIT         |
| `@angular/aria`                |    22.0.5 | Accessible headless interaction primitives                | MIT         |
| `ag-grid-angular`              |    36.0.1 | Angular adapter for complex lazy lists                    | MIT         |
| `ag-grid-community`            |    36.0.1 | Community-only grid, selectively registered               | MIT         |
| `livekit-client`               |    2.20.2 | Lazy-loaded optional WebRTC calls inside authorized chats | Apache-2.0  |
| `rxjs`                         |     7.8.2 | Cancellable async composition and SSE/Angular integration | Apache-2.0  |
| `tslib`                        |     2.8.1 | TypeScript runtime helpers                                | Apache-2.0  |
| `@veltrix-crm/contracts`      | workspace | Generated internal OpenAPI types; not third-party         | Project MIT |
| `@veltrix-crm/product-config` | workspace | Generated central brand/runtime config                    | Project MIT |

`ag-grid-enterprise` is forbidden. No second UI kit, Tailwind, Moment.js, full Lodash, icon font, remote font, or chart framework is installed.

## Production Go dependencies

| Dependency                        |           Version | Purpose                                                     | License      |
| --------------------------------- | ----------------: | ----------------------------------------------------------- | ------------ |
| Go standard library               | Go 1.26 toolchain | HTTP, crypto, JSON, logging, embed, streaming, concurrency  | BSD-3-Clause |
| `github.com/go-chi/chi/v5`        |             5.3.1 | Small HTTP router/middleware composition                    | MIT          |
| `github.com/golang-jwt/jwt/v5`    |             5.3.1 | Short-lived, room-scoped LiveKit participant token signing  | MIT          |
| `github.com/google/uuid`          |             1.6.0 | UUID parsing/interop; project generation uses UUIDv7 policy | BSD-3-Clause |
| `github.com/jackc/pgx/v5`         |            5.10.0 | PostgreSQL driver, pool, transactions, typed values         | MIT          |
| `github.com/oapi-codegen/runtime` |             1.6.0 | OpenAPI parameter/runtime helpers                           | Apache-2.0   |
| `golang.org/x/crypto`             |            0.54.0 | Argon2id and security primitives not in core stdlib         | BSD-3-Clause |

Direct code intentionally avoids an ORM, cache client, broker client, search client, metrics framework, and S3 SDK. The S3-compatible adapter uses bounded standard-library HTTP/SigV4 behavior.

## Required runtime infrastructure

| Component         | Pinned reference          | Purpose                                          | License                                                                        |
| ----------------- | ------------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------ |
| PostgreSQL        | `postgres:18.4-bookworm`  | Relational data, RLS, FTS/trigram, jobs, outbox  | PostgreSQL License; image also contains Debian components under their licenses |
| Application image | project `scratch` runtime | Static Go binary, healthcheck, CA bundle, assets | Project MIT plus bundled CA/license obligations                                |

The build stages use `node:24.15.0-bookworm-slim` and `golang:1.26.5-bookworm`, but neither toolchain is copied into runtime.

## Optional Compose profiles

These components are not required for base readiness and have different operational/license implications.

| Profile/component | Pinned reference                                   | Purpose                                   | License                                |
| ----------------- | -------------------------------------------------- | ----------------------------------------- | -------------------------------------- |
| Mailpit           | `axllent/mailpit:v1.30.4`                          | Local SMTP capture/UI                     | MIT                                    |
| MinIO             | `quay.io/minio/minio:RELEASE.2025-04-22T22-12-26Z` | Optional S3-compatible local object store | AGPL-3.0                               |
| Ollama            | `ollama/ollama:0.32.1`                             | Optional local AI-compatible endpoint     | MIT (review model licenses separately) |
| LiveKit           | `livekit/livekit-server:v1.12.0`                   | Optional self-hosted audio/video rooms    | Apache-2.0                              |
| Grafana k6        | `grafana/k6:2.1.0`                                 | Benchmark runner, not production          | AGPL-3.0                               |

AI model weights have independent licenses and are not distributed by this repository. OpenAI-compatible endpoints are configuration targets, not bundled dependencies.

## Build, test, and quality dependencies

| Dependency                     |                   Version | Purpose                                   | License                              |
| ------------------------------ | ------------------------: | ----------------------------------------- | ------------------------------------ |
| pnpm                           |                   11.15.1 | Reproducible JS workspace/install scripts | MIT                                  |
| Node.js                        |                   24.15.0 | Build/tool execution only                 | MIT plus bundled third-party notices |
| Angular CLI/build/compiler-cli |                    22.0.7 | Frontend compile/test/build               | MIT                                  |
| Playwright                     |                    1.61.1 | E2E, screenshots, browser scenarios       | Apache-2.0                           |
| axe-core Playwright            |                    4.12.1 | WCAG browser scans                        | MPL-2.0                              |
| `openapi-typescript`           |                    7.13.0 | Generate TypeScript API contracts         | MIT                                  |
| TypeScript                     |                     6.0.3 | Strict frontend compilation               | Apache-2.0                           |
| ESLint                         | 10.0.1 / 10.7.0 ecosystem | Static analysis                           | MIT                                  |
| Prettier                       |                     3.9.6 | Formatting                                | MIT                                  |
| Vitest                         |                    4.1.10 | Angular unit runner integration           | MIT                                  |
| jsdom                          |                    29.1.1 | Browser-like unit-test environment        | MIT                                  |

Go build/test tooling additionally uses `sqlc`, `go vet`, `govulncheck`, CodeQL, Trivy, and SBOM tooling through scripts/workflows; their pinned workflow/tool versions and licenses must be included in the generated CI SBOM/tool report.

## Dependency acceptance policy

Before adding a production dependency, document the current use case, payload/runtime cost, maintenance activity, license compatibility, security posture, and why standard APIs or existing dependencies are insufficient. Use stable releases and exact versions/lockfiles. Experimental/prerelease/deprecated APIs require an ADR.

CI should verify:

- frozen pnpm and Go module state;
- generated code is clean;
- `ag-grid-enterprise` and prohibited imports are absent;
- frontend production audit and `govulncheck` findings are reviewed, not bypassed;
- license policy and SBOM output;
- production runtime contains no Node.js;
- image vulnerabilities are scanned with severity/context recorded.

Automated scanners can miss issues and produce false positives. A green scan is one input to review, not proof of safety.
