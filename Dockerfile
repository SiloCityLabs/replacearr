# ==============================================================================
# Replacearr — Root Development Dockerfile
# Go + Node image for local containerized development (docker-compose.yml).
# Production image: docker/Dockerfile
# ==============================================================================
FROM golang:alpine

# Dev image uses Alpine-packaged nodejs/npm (unpinned). Host and CI use Node 24 (.nvmrc);
# production UI builds use node:24-alpine in docker/Dockerfile.
RUN apk add --no-cache \
    nodejs \
    npm \
    git \
    bash \
    curl \
    make \
    ca-certificates \
    tzdata

WORKDIR /workspace

# Cache Go module dependencies
COPY go.mod go.sum ./
RUN go mod download

# Cache frontend npm dependencies when present
COPY web/package*.json ./web/
RUN cd web && npm install

# Copy remaining project files (overlaid by bind mount in docker-compose.yml)
COPY . .

# Pre-build sample plugins and static frontend so a cold image can serve immediately
RUN make build-web build-plugins

ENV REPLACEARR_HOST=0.0.0.0 \
    REPLACEARR_PORT=8989 \
    REPLACEARR_CONFIG_DIR=/config \
    REPLACEARR_PLUGIN_DIR=/workspace/bin/plugins \
    REPLACEARR_DEFINITIONS_DIR=/config/definitions \
    REPLACEARR_ENV=development

EXPOSE 8989 5173

HEALTHCHECK --interval=15s --timeout=5s --start-period=90s --retries=8 \
    CMD curl -fsS http://localhost:8989/api/v1/system/health || exit 1

CMD ["./entrypoint.sh"]
