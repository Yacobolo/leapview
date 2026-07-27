# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e

FROM node:24-bookworm@sha256:392e1e23f34da768d8d1f4e502b64f200d3be3465934d4b7930f57d7e2fc1989 AS node

FROM golang:1.25-bookworm@sha256:a9c020ee3d1508c7be5435c262434e3d3fc1d0e76a11afeb9ddae7d60bc86aa4 AS sourcegen
WORKDIR /src

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    ./scripts/generate_build_sources.sh && \
    go run ./internal/app/tools/mapassets --out .data/map-assets && \
    go run ./internal/app/tools/clidocgen && \
    go run ./internal/app/tools/schemadocgen && \
    go run ./internal/app/tools/openapidocgen && \
    go run ./internal/app/tools/docsitegen

FROM oven/bun:1.3.7@sha256:6cd5f00020e48b77a253bc8249f6b6dd3d92b3c04c2607f1f5a6d7dbf0a6fca3 AS web
WORKDIR /src

COPY package.json bun.lock tsconfig.json ./
COPY scripts ./scripts
COPY static ./static
COPY web ./web
COPY --from=sourcegen /src/api/gen ./api/gen
COPY --from=sourcegen /src/api/visualization ./api/visualization
COPY --from=sourcegen /src/web/generated ./web/generated

RUN bun install --frozen-lockfile --no-cache
RUN bun scripts/generate_visualization_validator.ts && \
    bun scripts/generate_vega_lite_validator.ts && \
    bun run build

FROM golang:1.25-bookworm@sha256:a9c020ee3d1508c7be5435c262434e3d3fc1d0e76a11afeb9ddae7d60bc86aa4 AS build
WORKDIR /src

ARG BUILD_VERSION=development
ARG BUILD_REVISION=unknown
ARG BUILD_TIME=unknown
ARG BUILD_DIRTY=true
ARG BUILD_RELEASE=false

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=sourcegen /src/api/gen ./api/gen
COPY --from=sourcegen /src/internal/agent/api/gen ./internal/agent/api/gen
COPY --from=sourcegen /src/internal/app/api/aggregate ./internal/app/api/aggregate
COPY --from=sourcegen /src/internal/app/api/gen ./internal/app/api/gen
COPY --from=sourcegen /src/internal/platform/http/api/gen ./internal/platform/http/api/gen
COPY --from=sourcegen /src/internal/app/cli/gen ./internal/app/cli/gen
COPY --from=sourcegen /src/internal/app/config/config_gen.go ./internal/app/config/config_gen.go
COPY --from=sourcegen /src/internal/app/config/spec/names_gen.go ./internal/app/config/spec/names_gen.go
COPY --from=sourcegen /src/internal/platform/db/db.go ./internal/platform/db/db.go
COPY --from=sourcegen /src/internal/platform/db/models.go ./internal/platform/db/models.go
COPY --from=sourcegen /src/internal/platform/db/*.sql.go ./internal/platform/db/
COPY --from=sourcegen /src/internal/access/ui/signals/models.gen.go ./internal/access/ui/signals/models.gen.go
COPY --from=sourcegen /src/internal/admin/ui/signals/models.gen.go ./internal/admin/ui/signals/models.gen.go
COPY --from=sourcegen /src/internal/agent/ui/signals/models.gen.go ./internal/agent/ui/signals/models.gen.go
COPY --from=sourcegen /src/internal/dashboard/ui/signals/models.gen.go ./internal/dashboard/ui/signals/models.gen.go
COPY --from=sourcegen /src/internal/workspace/ui/signals/models.gen.go ./internal/workspace/ui/signals/models.gen.go
COPY --from=sourcegen /src/docs ./docs
COPY --from=sourcegen /src/schemas ./schemas
COPY --from=sourcegen /src/web/generated ./web/generated
COPY --from=web /src/static ./static

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    BUILD_LDFLAGS="-s -w \
      -X github.com/Yacobolo/leapview/internal/platform/buildinfo.version=${BUILD_VERSION} \
      -X github.com/Yacobolo/leapview/internal/platform/buildinfo.revision=${BUILD_REVISION} \
      -X github.com/Yacobolo/leapview/internal/platform/buildinfo.buildTime=${BUILD_TIME} \
      -X github.com/Yacobolo/leapview/internal/platform/buildinfo.dirty=${BUILD_DIRTY} \
      -X github.com/Yacobolo/leapview/internal/platform/buildinfo.release=${BUILD_RELEASE}" && \
    CGO_ENABLED=1 go build -tags=duckdb_arrow -trimpath -ldflags="$BUILD_LDFLAGS" -o /out/leapview ./cmd/leapview && \
    CGO_ENABLED=0 go build -trimpath -ldflags="$BUILD_LDFLAGS" -o /out/leapviewctl ./cmd/leapviewctl

FROM debian:bookworm-slim@sha256:60eac759739651111db372c07be67863818726f754804b8707c90979bda511df AS runtime

ARG BUILD_VERSION=development
ARG BUILD_REVISION=unknown
ARG BUILD_TIME=unknown
ARG BUILD_DIRTY=true
ARG BUILD_RELEASE=false

LABEL org.opencontainers.image.title="LeapView" \
      org.opencontainers.image.description="LeapView business intelligence server" \
      org.opencontainers.image.source="https://github.com/Yacobolo/leapview" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="$BUILD_VERSION" \
      org.opencontainers.image.revision="$BUILD_REVISION" \
      org.opencontainers.image.created="$BUILD_TIME" \
      dev.leapview.build.dirty="$BUILD_DIRTY" \
      dev.leapview.build.release="$BUILD_RELEASE"

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates libstdc++6 tzdata && \
    rm -rf /var/lib/apt/lists/*

RUN groupadd --system leapview && \
    useradd --system --gid leapview --home-dir /var/lib/leapview --shell /usr/sbin/nologin leapview

WORKDIR /app

COPY --from=build /out/leapview /usr/local/bin/leapview
COPY --from=build /out/leapviewctl /usr/local/libexec/leapviewctl
COPY --from=web /src/static ./static
COPY --from=build /src/schemas ./schemas
COPY --from=sourcegen /src/.data/map-assets ./.data/map-assets
COPY dashboards ./dashboards
COPY evaluation ./evaluation

RUN mkdir -p /var/lib/leapview && \
    chown -R leapview:leapview /var/lib/leapview /app

USER leapview

ENV LEAPVIEW_ADDR=:8080 \
    LEAPVIEW_ENVIRONMENT=prod \
    LEAPVIEW_HOME=/var/lib/leapview/home \
    LEAPVIEW_MAP_ASSET_DIR=/app/.data/map-assets \
    LEAPVIEW_MANAGED_DATA_DIR=/var/lib/leapview/home/managed-data \
    LEAPVIEW_PRODUCTION=1

EXPOSE 8080
VOLUME ["/var/lib/leapview"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["leapview", "healthcheck"]

ENTRYPOINT ["leapview"]
CMD ["serve", "--production"]
