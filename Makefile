SHELL := /bin/bash
.DEFAULT_GOAL := help

APP_NAME := replacearr
VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

BIN_DIR := bin
PLUGINS_DIR := $(BIN_DIR)/plugins
WEB_DIR := web
EMBED_DIST := internal/webui/dist

.PHONY: help \
        start stop build-docker \
        build-web build-backend build-plugins build test test-go test-web \
        sync-definitions \
        clean clean-all

help: ## Show all available make targets
	@echo "Replacearr — Unified Sonarr, Radarr & Prowlarr Replacement"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

start: ## Start containerized local dev via root docker-compose.yml (/data/TV paths, Vite+API)
	@mkdir -p config data/TV data/Movies data/downloads
	docker compose up --build -d

stop: ## Stop local containerized stack (keeps ./config and ./data)
	docker compose down

# Sparse-clone Prowlarr/Indexers definitions/v11 into ./definitions (Replacearr-owned catalog).
# Existing files under ./definitions are replaced; runtime copies under ./config/definitions are untouched.
sync-definitions: ## Refresh bundled Cardigann YAML definitions from Prowlarr/Indexers (v11)
	@echo "==> Syncing indexer YAML definitions from Prowlarr/Indexers (definitions/v11)..."
	@tmpdir=$$(mktemp -d) && \
	  git clone --depth 1 --filter=blob:none --sparse https://github.com/Prowlarr/Indexers.git "$$tmpdir/Indexers" && \
	  git -C "$$tmpdir/Indexers" sparse-checkout set definitions/v11 && \
	  mkdir -p definitions && \
	  rsync -a --delete --include='*.yml' --include='*.yaml' --include='schema.json' --exclude='*' "$$tmpdir/Indexers/definitions/v11"/ definitions/ && \
	  commit=$$(git -C "$$tmpdir/Indexers" rev-parse --short HEAD) && \
	  date=$$(date -u +%Y-%m-%d) && \
	  printf '%s\n' \
	    '# Indexer YAML definitions' \
	    '' \
	    'These Cardigann YAML indexer definitions are maintained for Replacearr under `definitions/`.' \
	    '' \
	    'They were originally sourced from [Prowlarr/Indexers](https://github.com/Prowlarr/Indexers) (`definitions/v11`), which itself syncs many definitions from [Jackett/Jackett](https://github.com/Jackett/Jackett).' \
	    '' \
	    "- Upstream snapshot: Prowlarr/Indexers@$$commit (synced $$date)" \
	    '- Schema: `schema.json` (Cardigann v11)' \
	    '' \
	    'Refresh with `make sync-definitions`. Custom or edited definitions in `$$REPLACEARR_CONFIG_DIR/definitions` are preserved on seed (existing files are not overwritten).' \
	    > definitions/NOTICE.md && \
	  rm -rf "$$tmpdir" && \
	  echo "==> Synced $$(ls definitions/*.yml 2>/dev/null | wc -l) YAML definitions into definitions/"

build-docker: ## Build public production Docker image from docker/Dockerfile
	docker build -f docker/Dockerfile -t ldrrp/replacearr:latest -t ldrrp/replacearr:$(VERSION) .

build-web: ## Build static React frontend and copy into Go embed directory
	@echo "==> Building static React frontend..."
	cd $(WEB_DIR) && npm run build
	@rm -rf $(EMBED_DIST)
	@mkdir -p $(EMBED_DIST)
	@cp -r $(WEB_DIR)/dist/* $(EMBED_DIST)/
	@touch $(EMBED_DIST)/.gitkeep
	@echo "==> Static frontend embedded in $(EMBED_DIST)"

build-plugins: ## Build local example Go plugins into bin/plugins (excludes sample-indexer demo)
	@echo "==> Building Go plugins..."
	@mkdir -p $(PLUGINS_DIR)
	CGO_ENABLED=0 go build -buildvcs=false -ldflags="$(LDFLAGS)" -o $(PLUGINS_DIR)/openvpn-proxy ./examples/plugins/openvpn-proxy
	CGO_ENABLED=0 go build -buildvcs=false -ldflags="$(LDFLAGS)" -o $(PLUGINS_DIR)/webhook-notifier ./examples/plugins/webhook-notifier

build-backend: ## Build Replacearr Go binary (assumes build-web has been run)
	@echo "==> Building Replacearr binary..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(EMBED_DIST)
	@touch $(EMBED_DIST)/.gitkeep
	CGO_ENABLED=0 go build -buildvcs=false -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/replacearr

build: build-web build-plugins build-backend ## Full production build: static React UI + plugins + single Go binary
	@echo "==> Build complete: $(BIN_DIR)/$(APP_NAME) and $(PLUGINS_DIR)/*"

test: test-go test-web ## Run all backend and frontend tests

test-go: build-web ## Run Go unit and integration tests (embeds built SPA)
	@echo "==> Running Go tests..."
	go test -v -race ./...

test-web: ## Run frontend typecheck and build verification
	@echo "==> Typechecking frontend..."
	cd $(WEB_DIR) && npm run typecheck

clean: ## Remove build artifacts and embedded dist files
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BIN_DIR) $(WEB_DIR)/dist coverage.out
	find $(EMBED_DIST) -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true

clean-all: clean ## Also remove web/node_modules
	@echo "==> Deep cleaning (node_modules)..."
	rm -rf $(WEB_DIR)/node_modules
