# Replacearr

**Replacearr** is a fast, single-binary, Go-powered replacement for **Sonarr**, **Radarr**, and **Prowlarr** with a unified React web interface, role-based access control (RBAC) for users and scoped API keys, native Cardigann YAML indexer definitions, and an out-of-process **Go Plugin SDK** (`pkg/pluginsdk`).

## Screenshots

![Sign in](images/00-login.png)
![Dashboard](images/01-dashboard.png)
![TV series](images/02-series.png)
![Movies](images/03-movies.png)
![Indexers and search](images/04-indexers-search.png)
![Queue and download clients](images/05-queue.png)
![Plugins](images/06-plugins.png)
![Users and roles](images/07-users.png)
![Settings](images/08-settings.png)
![General settings](images/09-settings-general.png)
![UI settings](images/10-settings-ui.png)
![Media management](images/11-settings-media.png)
![Profiles](images/12-settings-profiles.png)
![Quality definitions](images/13-settings-quality.png)
![Indexer settings](images/14-settings-indexers.png)
![Download client settings](images/15-settings-download-clients.png)
![Import lists](images/16-settings-import-lists.png)
![Connect](images/17-settings-connect.png)
![Metadata](images/18-settings-metadata.png)
![Tags](images/19-settings-tags.png)
![Compatibility hub](images/20-compatibility.png)

---

## Key Features

- **Unified Media & Indexer Engine**: Manage TV series (Sonarr), Movies (Radarr), Indexers & Search (Prowlarr), Download Clients, Quality Profiles, Root Folders, Calendar, Activity Queue, and History in a single lightweight process and SQLite database.
- **Drop-In Upstream `*arr` API Compatibility**:
  - **Sonarr v3 API**: `/sonarr/api/v3/*` (and `/api/v3/*` auto-routed)
  - **Radarr v3 API**: `/radarr/api/v3/*` (and `/api/v3/*` auto-routed)
  - **Prowlarr v1 API**: `/prowlarr/api/v1/*` (and `/api/v1/*` auto-routed)
  - Connect Overseerr, Jellyseerr, Bazarr, Homepage, Tautulli, or mobile apps (LunaSea, nzb360) directly to Replacearr without changing client code.
- **Multi-User RBAC & Scoped API Keys**:
  - Built-in roles (`admin`, `manager`, `requester`, `viewer`, `integration`) and custom roles with granular permissions (`tv:read`, `tv:write`, `movies:write`, `indexers:search`, `plugins:manage`, `users:manage`, etc.).
  - Create API keys scoped to `all`, `sonarr`, `radarr`, or `prowlarr` with fine-grained permission subsets.
  - Supports JWT Bearer tokens (`Authorization: Bearer <token>`) as well as `*arr`-compatible `X-Api-Key` headers and `?apikey=` query parameters.
- **Go Plugin SDK (`pkg/pluginsdk`)**:
  - Extend Replacearr with custom indexers, download clients, or notification hooks written in standard Go.
  - Out-of-process JSON-RPC 2.0 over stdio isolation — plugins run as independent binaries in `/config/plugins` without requiring CGO or dynamic library version lock-in.
- **Cardigann YAML Definition Loader**:
  - Loads Prowlarr/Jackett-compatible Cardigann YAML indexer definitions directly from `/config/definitions` (or `.tmp/ProwlarrIndexers/definitions/v11` in development).
- **Single Static Binary & LinuxServer-Style Docker Image**:
  - Pure-Go SQLite (`modernc.org/sqlite`), embedded React SPA (`internal/webui`), `CGO_ENABLED=0` static compilation, and a production multi-stage container supporting `PUID`/`PGID` and `TZ`.

---

## Architecture Overview

```text
replacearr/
├── cmd/replacearr/                # Main server entrypoint
├── examples/plugins/
│   └── sample-indexer/            # Sample out-of-process Go indexer plugin
├── internal/
│   ├── api/                       # Unified REST API + Sonarr/Radarr/Prowlarr compat routers
│   ├── auth/                      # Password hashing, JWT tokens, RBAC & API key middleware
│   ├── config/                    # Environment & file-backed runtime configuration
│   ├── db/                        # Pure-Go SQLite store, migrations & default seeders
│   ├── domain/                    # Core domain models & RBAC permission constants
│   ├── indexers/                  # Torznab/Newznab, Cardigann YAML & Plugin search engine
│   ├── plugins/                   # Out-of-process plugin host & JSON-RPC supervisor
│   └── webui/                     # Go embed.FS serving the compiled React SPA
├── pkg/pluginsdk/                 # Public Go SDK for authoring Replacearr plugins
├── web/                           # React + TypeScript + Vite unified frontend
├── docker/
│   ├── Dockerfile                 # Multi-stage production container image
│   ├── docker-compose.prod.yml    # End-user production Docker Compose stack
│   └── entrypoint.sh              # PUID/PGID setup & bundled plugin seeding
├── Dockerfile                     # Root development & CI container
├── docker-compose.yml             # Development & integration testing Compose file
└── Makefile                       # Build, dev, test, and Docker automation
```

---

## Upstream `*arr` Compatibility Endpoints

Replacearr exposes both a unified modern API (`/api/v1/*`) for its web UI and drop-in compatibility prefixes so external ecosystem tools can treat a single Replacearr instance as Sonarr, Radarr, and Prowlarr simultaneously:

| Ecosystem Service | Base URL Prefix | Supported Endpoints |
| :--- | :--- | :--- |
| **Sonarr v3** | `/sonarr/api/v3` | `/system/status`, `/series`, `/series/lookup`, `/episode`, `/qualityprofile`, `/rootfolder`, `/languageprofile`, `/tag`, `/queue`, `/calendar`, `/command`, `/downloadclient`, `/history` |
| **Radarr v3** | `/radarr/api/v3` | `/system/status`, `/movie`, `/movie/lookup`, `/qualityprofile`, `/rootfolder`, `/tag`, `/queue`, `/calendar`, `/command`, `/downloadclient`, `/history` |
| **Prowlarr v1** | `/prowlarr/api/v1` | `/system/status`, `/indexer`, `/indexer/schema`, `/search`, `/tag`, `/downloadclient`, `/health` |
| **Shared Root** | `/api/v3`, `/api/v1` | Direct root-level compatibility routes for `/api/v3/series`, `/api/v3/movie`, `/api/v1/indexer`, `/api/v1/search`, `/api` |

### Connecting Overseerr / Jellyseerr / Bazarr

1. In Replacearr, navigate to **Users & RBAC → API Keys**.
2. Create an API key with the **Integration** role (or custom permissions) and select a scope (`all`, `sonarr`, `radarr`, or `prowlarr`).
3. Configure your external tool:
   - **Sonarr URL**: `http://replacearr:8989/sonarr` (or `http://replacearr:8989` using root `/api/v3`)
   - **Radarr URL**: `http://replacearr:8989/radarr` (or `http://replacearr:8989` using root `/api/v3`)
   - **Prowlarr URL**: `http://replacearr:8989/prowlarr`

---

## Role-Based Access Control (RBAC) & Authentication

Replacearr supports multi-user accounts and scoped API keys out of the box.

### Built-In Roles

| Role | Description | Default Permissions |
| :--- | :--- | :--- |
| `admin` | Full administrative access | `*` (all permissions) |
| `manager` | Manage media, indexers, and downloads | `tv:*`, `movies:*`, `indexers:*`, `downloads:*`, `profiles:*`, `plugins:read`, `system:read` |
| `requester` | Add TV series/movies and search indexers | `tv:read`, `tv:write`, `movies:read`, `movies:write`, `indexers:read`, `indexers:search`, `downloads:read`, `profiles:read`, `system:read` |
| `viewer` | Read-only access to library and queue | `tv:read`, `movies:read`, `indexers:read`, `downloads:read`, `profiles:read`, `system:read` |
| `integration` | Service role for `*arr` apps | `tv:read`, `tv:write`, `movies:read`, `movies:write`, `indexers:read`, `indexers:search`, `downloads:read`, `downloads:write`, `profiles:read`, `system:read` |

### Permission Scopes

- **Media**: `tv:read`, `tv:write`, `tv:delete`, `movies:read`, `movies:write`, `movies:delete`
- **Indexers**: `indexers:read`, `indexers:write`, `indexers:search`
- **Downloads & Profiles**: `downloads:read`, `downloads:write`, `profiles:read`, `profiles:write`
- **Administration**: `plugins:read`, `plugins:manage`, `users:read`, `users:manage`, `system:read`, `system:manage`

---

## Go Plugin SDK (`pkg/pluginsdk`)

Replacearr features an extensible, out-of-process plugin system using JSON-RPC 2.0 over standard input/output. Anyone can write a plugin in pure Go without compiling against internal Replacearr packages or worrying about shared-library (`go plugin`) version mismatches.

### Writing a Custom Indexer Plugin

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type CustomIndexer struct {
	settings map[string]any
}

func (p *CustomIndexer) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          "com.example.custom-indexer",
		Name:        "Custom Go Indexer",
		Version:     "1.0.0",
		Author:      "Your Name",
		Description: "Example custom indexer built with Replacearr Go Plugin SDK",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeIndexer},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "baseUrl",
				Label:    "Tracker URL",
				Type:     "text",
				Default:  "https://tracker.example.com",
				Required: true,
			},
			{
				Name:     "apiKey",
				Label:    "API Key",
				Type:     "password",
				Required: false,
			},
		},
	}
}

func (p *CustomIndexer) Configure(settings map[string]any) error {
	p.settings = settings
	return nil
}

func (p *CustomIndexer) Search(_ context.Context, req pluginsdk.SearchRequest) ([]pluginsdk.Release, error) {
	query := req.Query
	if query == "" {
		query = "Latest"
	}
	return []pluginsdk.Release{
		{
			Guid:        fmt.Sprintf("custom-%d", time.Now().UnixNano()),
			Title:       fmt.Sprintf("%s.2026.1080p.WEB-DL.DDP5.1.H.264-REPLACEARR", query),
			Protocol:    "torrent",
			Size:        4_294_967_296,
			PublishDate: time.Now().UTC().Format(time.RFC3339),
			DownloadURL: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
			Seeders:     150,
			Peers:       165,
			Categories:  []int{2000, 5000},
			Quality:     "WEBDL-1080p",
		},
	}, nil
}

func main() {
	pluginsdk.Serve(&CustomIndexer{})
}
```

Build your plugin as a standard Go executable and place it in `/config/plugins` (or `bin/plugins` in development):

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o /config/plugins/my-indexer ./cmd/my-indexer
```

Then click **Reload Plugins** in the **Plugins** tab of the Replacearr UI.

---

## Docker Deployment

### Production Deployment (`docker/docker-compose.prod.yml`)

Run Replacearr in production using the multi-stage container in `docker/`:

```bash
# Build and start the production container in the background
make docker-prod-run
# Or directly with Docker Compose:
docker compose -f docker/docker-compose.prod.yml up -d --build
```

Example `docker-compose.yml` for end users:

```yaml
services:
  replacearr:
    image: ghcr.io/replacearr/replacearr:latest
    container_name: replacearr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=UTC
      - REPLACEARR_ADMIN_USER=admin
      - REPLACEARR_ADMIN_PASS=replacearr
    ports:
      - "8989:8989"
    volumes:
      - ./config:/config
      - ./media:/data
    restart: unless-stopped
```

### Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PUID` | `1000` | User ID for file permissions inside the container |
| `PGID` | `1000` | Group ID for file permissions inside the container |
| `TZ` | `UTC` | Container timezone |
| `REPLACEARR_HOST` | `0.0.0.0` | HTTP bind address |
| `REPLACEARR_PORT` | `8989` | HTTP listen port |
| `REPLACEARR_CONFIG_DIR` | `/config` | Directory for SQLite database, JWT secret, and plugins |
| `REPLACEARR_DB_PATH` | `/config/replacearr.db` | Custom path to SQLite database file |
| `REPLACEARR_PLUGIN_DIR` | `/config/plugins` | Directory scanned for executable Go plugins |
| `REPLACEARR_DEFINITIONS_DIR` | `/config/definitions` | Directory scanned for Cardigann YAML definitions |
| `REPLACEARR_ADMIN_USER` | `admin` | Initial admin username seeded on first startup |
| `REPLACEARR_ADMIN_PASS` | `replacearr` | Initial admin password seeded on first startup |
| `REPLACEARR_API_KEY` | *(auto-generated)* | Optional fixed default system API key |
| `REPLACEARR_JWT_SECRET` | *(auto-generated)* | Secret key used for signing JWT tokens |

---

## Development & Makefile Commands

### Prerequisites

- **Go** `1.23+` (tested with Go `1.26`)
- **Node.js** `20+` and **npm**
- **Docker** & **Docker Compose** (optional, for containerized workflows)

### Quick Start (Local Development)

```bash
# 1. Install Go + Web dependencies and shallow-clone upstream reference repos into .tmp/
make setup

# 2. Run Go backend (:8989) + Vite React dev server (:5173) concurrently
make dev

# Or build a single self-contained binary with embedded UI + plugins:
make build
./bin/replacearr
```

### Makefile Targets Reference

| Command | Description |
| :--- | :--- |
| `make help` | Show all available `make` targets |
| `make setup` | Install Go/npm dependencies and clone Sonarr, Radarr, Prowlarr & Indexers into `.tmp/` |
| `make setup-refs` | Shallow-clone upstream Sonarr, Radarr, Prowlarr, and Indexers into `.tmp/` |
| `make dev` | Run Go backend (`:8989`) and Vite React frontend (`:5173`) concurrently |
| `make dev-backend` | Build plugins and run only the Go backend server |
| `make dev-web` | Run only the React Vite development server |
| `make build-web` | Build static React SPA and copy into `internal/webui/dist` |
| `make build-plugins` | Compile bundled Go plugins into `bin/plugins/` |
| `make build-backend` | Compile the static `bin/replacearr` binary |
| `make build` | Full production build (React UI + plugins + static Go binary) |
| `make test` | Run all Go backend unit/integration tests and frontend typechecks |
| `make test-go` | Run `go test -v -race ./...` |
| `make test-web` | Run TypeScript verification (`npm run typecheck`) in `web/` |
| `make docker-dev` | Build and start the root development Docker Compose environment |
| `make docker-dev-down` | Stop and remove the development Docker Compose containers |
| `make docker-prod` | Build the public production multi-stage Docker image (`docker/Dockerfile`) |
| `make docker-prod-run` | Start the production stack using `docker/docker-compose.prod.yml` |
| `make clean` | Clean build artifacts, embedded dist files, and temporary dev config |
| `make clean-all` | Deep clean including `.tmp/` reference repos and `web/node_modules` |

---

## License

MIT
