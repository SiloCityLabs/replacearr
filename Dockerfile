# ==============================================================================
# Replacearr — Root Development Dockerfile
# Provides a full Go + Node.js development & integration testing environment
# ==============================================================================
FROM golang:alpine

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

# Copy remaining project files
COPY . .

# Pre-build bundled sample plugins and static frontend so backend works immediately
RUN make build-web build-plugins

ENV REPLACEARR_HOST=0.0.0.0 \
    REPLACEARR_PORT=8989 \
    REPLACEARR_CONFIG_DIR=/config \
    REPLACEARR_PLUGIN_DIR=/workspace/bin/plugins \
    REPLACEARR_ENV=development

EXPOSE 8989 5173

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=5 \
    CMD curl -fsS http://localhost:8989/api/v1/system/health || exit 1

CMD ["sh", "-c", "make dev-backend CONFIG_DIR=${REPLACEARR_CONFIG_DIR:-/config}"]
