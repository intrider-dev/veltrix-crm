# syntax=docker/dockerfile:1.7

ARG NODE_IMAGE=node:24.15.0-bookworm-slim
ARG GO_IMAGE=golang:1.26.5-bookworm

FROM ${NODE_IMAGE} AS web-deps

ENV PNPM_HOME=/pnpm \
    PATH=/pnpm:$PATH
WORKDIR /src

RUN corepack enable && corepack prepare pnpm@11.15.1 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/package.json
COPY packages/contracts/package.json packages/contracts/package.json
COPY packages/i18n/package.json packages/i18n/package.json
COPY packages/product-config/package.json packages/product-config/package.json

RUN --mount=type=cache,id=veltrix-pnpm,target=/pnpm/store \
    pnpm config set store-dir /pnpm/store && pnpm install --frozen-lockfile

FROM web-deps AS web-build

COPY apps/web apps/web
COPY packages packages
COPY infra/container/precompress.mjs infra/container/precompress.mjs

RUN pnpm --filter @veltrix-crm/web build
RUN node infra/container/precompress.mjs apps/web/dist/web/browser

FROM ${GO_IMAGE} AS api-build

WORKDIR /src

COPY go.work go.work.sum ./
COPY apps/api/go.mod apps/api/go.sum apps/api/
RUN --mount=type=cache,id=veltrix-go-mod,target=/go/pkg/mod \
    go -C apps/api mod download

COPY apps/api apps/api
COPY infra/container/healthcheck.go infra/container/healthcheck.go
COPY packages/product-config/product.json packages/product-config/product.json

RUN rm -rf apps/api/internal/platform/webui/assets
COPY --from=web-build /src/apps/web/dist/web/browser apps/api/internal/platform/webui/assets

RUN mkdir -p /out
RUN --mount=type=cache,id=veltrix-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 go -C apps/api build -tags timetzdata -trimpath \
      -ldflags="-s -w -buildid=" -o /out/veltrix-crm ./cmd/server
RUN --mount=type=cache,id=veltrix-go-health-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" \
      -o /out/healthcheck infra/container/healthcheck.go
RUN mkdir -p /out/config /out/uploads /out/tmp && \
    cp packages/product-config/product.json /out/config/brand.json

FROM scratch AS runtime

COPY --from=api-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=api-build --chown=65532:65532 /out/veltrix-crm /app/veltrix-crm
COPY --from=api-build --chown=65532:65532 /out/healthcheck /app/healthcheck
COPY --from=api-build --chown=65532:65532 /out/config /app/config
COPY --from=api-build --chown=65532:65532 /out/uploads /var/lib/veltrix-crm/uploads
COPY --from=api-build --chown=65532:65532 /out/tmp /tmp

ENV APP_BRAND_CONFIG=/app/config/brand.json \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt \
    TMPDIR=/tmp

WORKDIR /app
USER 65532:65532
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD ["/app/healthcheck", "-url", "http://127.0.0.1:8080/api/v1/health/live", "-timeout", "2s"]

ENTRYPOINT ["/app/veltrix-crm"]
CMD ["serve"]
