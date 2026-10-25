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
TMP_DIR := .tmp
CONFIG_DIR := $(TMP_DIR)/dev-config

.PHONY: help setup setup-refs setup-go setup-web dev dev-backend dev-web \
        build-web build-backend build-plugins build test test-go test-web \
        lint docker-dev docker-dev-down docker-prod docker-prod-run \
        clean clean-all

help: ## Show all available make targets
	@echo "Replacearr — Unified Sonarr, Radarr & Prowlarr Replacement"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

setup: setup-refs setup-go setup-web ## Install all dependencies and clone upstream *arr reference repos into .tmp

setup-refs: ## Shallow-clone Sonarr, Radarr, Prowlarr, Indexers, and FlareSolverr into .tmp for reference
	@mkdir -p $(TMP_DIR)
	@if [ ! -d "$(TMP_DIR)/Sonarr" ]; then \
		echo "==> Cloning Sonarr into $(TMP_DIR)/Sonarr..."; \
		git clone --depth 1 https://github.com/Sonarr/Sonarr.git $(TMP_DIR)/Sonarr; \
	else \
		echo "==> $(TMP_DIR)/Sonarr already exists"; \
	fi
	@if [ ! -d "$(TMP_DIR)/Radarr" ]; then \
		echo "==> Cloning Radarr into $(TMP_DIR)/Radarr..."; \
		git clone --depth 1 https://github.com/Radarr/Radarr.git $(TMP_DIR)/Radarr; \
	else \
		echo "==> $(TMP_DIR)/Radarr already exists"; \
	fi
	@if [ ! -d "$(TMP_DIR)/Prowlarr" ]; then \
		echo "==> Cloning Prowlarr into $(TMP_DIR)/Prowlarr..."; \
		git clone --depth 1 https://github.com/Prowlarr/Prowlarr.git $(TMP_DIR)/Prowlarr; \
	else \
		echo "==> $(TMP_DIR)/Prowlarr already exists"; \
	fi
	@if [ ! -d "$(TMP_DIR)/ProwlarrIndexers" ]; then \
		echo "==> Cloning Prowlarr Indexers into $(TMP_DIR)/ProwlarrIndexers..."; \
		git clone --depth 1 https://github.com/Prowlarr/Indexers.git $(TMP_DIR)/ProwlarrIndexers; \
	else \
		echo "==> $(TMP_DIR)/ProwlarrIndexers already exists"; \
	fi
	@if [ ! -d "$(TMP_DIR)/FlareSolverr" ]; then \
		echo "==> Cloning FlareSolverr into $(TMP_DIR)/FlareSolverr..."; \
		git clone --depth 1 https://github.com/FlareSolverr/FlareSolverr.git $(TMP_DIR)/FlareSolverr; \
	else \
		echo "==> $(TMP_DIR)/FlareSolverr already exists"; \
	fi

setup-go: ## Download and tidy Go module dependencies
	@echo "==> Tidying Go modules..."
	go mod tidy

setup-web: ## Install frontend npm dependencies
	@echo "==> Installing frontend dependencies..."
	cd $(WEB_DIR) && npm install

dev: build-plugins ## Run Go backend and Vite React frontend concurrently for development
	@mkdir -p $(CONFIG_DIR)
	@echo "==> Starting Replacearr backend (:8989) and Vite dev server (:5173)..."
	@trap 'kill 0' INT TERM; \
	(REPLACEARR_CONFIG_DIR=$(CONFIG_DIR) REPLACEARR_PLUGIN_DIR=$(PLUGINS_DIR) REPLACEARR_DEFINITIONS_DIR=$(TMP_DIR)/ProwlarrIndexers/definitions/v11 go run ./cmd/replacearr) & \
	(cd $(WEB_DIR) && npm run dev) & \
	wait

dev-backend: build-plugins ## Run only the Go backend in development mode
	@mkdir -p $(CONFIG_DIR)
	REPLACEARR_CONFIG_DIR=$(CONFIG_DIR) REPLACEARR_PLUGIN_DIR=$(PLUGINS_DIR) REPLACEARR_DEFINITIONS_DIR=$(TMP_DIR)/ProwlarrIndexers/definitions/v11 go run ./cmd/replacearr

dev-web: ## Run only the React Vite dev server
	cd $(WEB_DIR) && npm run dev

build-web: ## Build static React frontend and copy into Go embed directory
	@echo "==> Building static React frontend..."
	cd $(WEB_DIR) && npm run build
	@rm -rf $(EMBED_DIST)
	@mkdir -p $(EMBED_DIST)
	@cp -r $(WEB_DIR)/dist/* $(EMBED_DIST)/
	@touch $(EMBED_DIST)/.gitkeep
	@echo "==> Static frontend embedded in $(EMBED_DIST)"

build-plugins: ## Build bundled and sample Go plugins into bin/plugins
	@echo "==> Building Go plugins..."
	@mkdir -p $(PLUGINS_DIR)
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(PLUGINS_DIR)/sample-indexer ./examples/plugins/sample-indexer

build-backend: ## Build Replacearr Go binary (assumes build-web has been run)
	@echo "==> Building Replacearr binary..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(EMBED_DIST)
	@touch $(EMBED_DIST)/.gitkeep
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/replacearr

build: build-web build-plugins build-backend ## Full production build: static React UI + plugins + single Go binary
	@echo "==> Build complete: $(BIN_DIR)/$(APP_NAME) and $(PLUGINS_DIR)/*"

test: test-go test-web ## Run all backend and frontend tests

test-go: build-web ## Run Go unit and integration tests (embeds built SPA)
	@echo "==> Running Go tests..."
	go test -v -race ./...

test-web: ## Run frontend typecheck and build verification
	@echo "==> Typechecking frontend..."
	cd $(WEB_DIR) && npm run typecheck

docker-dev: ## Start development testing environment with Docker Compose
	docker compose -f docker-compose.yml up --build

docker-dev-down: ## Stop and remove development Docker Compose containers
	docker compose -f docker-compose.yml down -v

docker-prod: ## Build public production Docker image from docker/Dockerfile
	docker build -f docker/Dockerfile -t replacearr:latest -t replacearr:$(VERSION) .

docker-prod-run: ## Start production Docker stack using docker/docker-compose.prod.yml
	docker compose -f docker/docker-compose.prod.yml up -d --build

clean: ## Remove build artifacts, embedded dist files, and temporary dev config
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BIN_DIR) $(WEB_DIR)/dist $(CONFIG_DIR) coverage.out
	find $(EMBED_DIST) -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true

clean-all: clean ## Remove all build artifacts, node_modules, and cloned .tmp reference repos
	@echo "==> Deep cleaning (.tmp and node_modules)..."
	rm -rf $(TMP_DIR) $(WEB_DIR)/node_modules
