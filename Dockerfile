# Default Vivy organism as one container: embedded UI, SQLite on /data.
# replica=1. Do not publish 8787 on all host interfaces.

ARG GOPROXY=https://goproxy.cn,direct
ARG NPM_REGISTRY=https://registry.npmmirror.com

FROM node:22-bookworm-slim AS ui
ARG NPM_REGISTRY
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
WORKDIR /src/ui
RUN corepack enable \
    && corepack prepare pnpm@10.33.0 --activate
COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm config set registry "${NPM_REGISTRY}" \
    && pnpm config set dangerouslyAllowAllBuilds true \
    && pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm build

FROM golang:1.26-bookworm AS build
ARG GOPROXY
ENV CGO_ENABLED=0 GO111MODULE=on GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/ui/dist ./ui/dist
RUN go build -trimpath -ldflags "-s -w" -o /out/vivy ./cmd/vivy

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates
WORKDIR /app
COPY --from=build /out/vivy /app/vivy
COPY docker/ /app/docker/
COPY fixtures/provider /app/fixtures/provider
ENV VIVY_CONFIG=/app/docker/config.yaml \
    VIVY_ADDR=0.0.0.0:8787 \
    TZ=Asia/Shanghai
EXPOSE 8787
VOLUME /data
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=5 \
    CMD wget -q -O /dev/null --tries=1 http://127.0.0.1:8787/healthz || exit 1
ENTRYPOINT ["/app/vivy"]
