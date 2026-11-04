# ==============================================================================
# Replacearr — Public Production Multi-Stage Dockerfile
# Stage 1: Build React SPA Frontend
# Stage 2: Build Static Go Binary + Bundled Go Plugins
# Stage 3: Minimal Alpine Linux Runtime with PUID/PGID Support
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Web UI Builder
# ------------------------------------------------------------------------------
FROM node:24-alpine AS web-builder

WORKDIR /app/web
COPY web/package*.json ./
RUN npm install

COPY web/ ./
RUN npm run build

# ------------------------------------------------------------------------------
# Stage 2: Go Binary & Plugin Builder
# ------------------------------------------------------------------------------
FROM golang:alpine AS go-builder

ARG VERSION=0.1.0
ARG COMMIT=prod

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Embed compiled React frontend into internal/webui/dist
RUN rm -rf internal/webui/dist && mkdir -p internal/webui/dist
COPY --from=web-builder /app/web/dist/ ./internal/webui/dist/

# Compile main Replacearr static binary.
# Example plugins (including the Torznab demo indexer) stay under examples/
# and are not bundled into the production image.
RUN mkdir -p /out/plugins && \
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    CGO_ENABLED=0 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
      -o /out/replacearr ./cmd/replacearr

# ------------------------------------------------------------------------------
# Stage 3: Production Runtime
# ------------------------------------------------------------------------------
FROM alpine:3.20 AS runtime

LABEL org.opencontainers.image.title="Replacearr" \
      org.opencontainers.image.description="Unified Sonarr, Radarr, and Prowlarr replacement with RBAC and Go Plugin SDK" \
      org.opencontainers.image.source="https://github.com/replacearr/replacearr" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    su-exec \
    curl && \
    mkdir -p \
      /config \
      /config/plugins \
      /config/definitions \
      /data/TV \
      /data/Movies \
      /data/Music \
      /data/Books \
      /data/Audiobooks \
      /data/downloads \
      /usr/local/share/replacearr/plugins \
      /usr/local/share/replacearr/definitions

COPY --from=go-builder /out/replacearr /usr/local/bin/replacearr
COPY --from=go-builder /out/plugins/ /usr/local/share/replacearr/plugins/
COPY definitions/ /usr/local/share/replacearr/definitions/
COPY docker/entrypoint.sh /entrypoint.sh

RUN chmod +x /usr/local/bin/replacearr /entrypoint.sh

ENV PUID=1000 \
    PGID=1000 \
    TZ=UTC \
    REPLACEARR_HOST=0.0.0.0 \
    REPLACEARR_PORT=8989 \
    REPLACEARR_CONFIG_DIR=/config \
    REPLACEARR_PLUGIN_DIR=/config/plugins \
    REPLACEARR_ENV=production

EXPOSE 8989

VOLUME ["/config", "/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8989/api/v1/system/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
