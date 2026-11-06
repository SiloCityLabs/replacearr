# AGENTS.md

Guidance for coding agents working in this repository. Persistent Cursor rules live in `.cursor/rules/` and apply automatically; this file is the map.

`docs/discovery/` is the living record of how Replacearr maps onto Sonarr, Radarr, and Prowlarr. When a discovery note disagrees with the code, follow the code and update the note.

## What this repo is

Replacearr is a single-binary Go replacement for Sonarr, Radarr, and Prowlarr. One SQLite database, an embedded React SPA, drop-in `*arr` HTTP APIs, and out-of-process Go plugins.

| Path | Role |
| :--- | :--- |
| `VERSION` | Product version source of truth (build-time inject into `main.Version`) |
| `cmd/replacearr` | Server main |
| `internal/api/native` | Unified `/api/v1` used by the web UI |
| `internal/api/compat/{sonarr,radarr,prowlarr}` | Wire-compatible upstream APIs |
| `internal/auth` | Passwords, JWT, RBAC, API keys |
| `internal/db` | SQLite store and SQL migrations |
| `internal/domain` | Models and permission constants |
| `internal/flaresolverr` | FlareSolverr v1 client, challenge detection, clearance cache |
| `pkg/pluginsdk` | Public plugin SDK |
| `internal/plugincatalog` | Marketplace `registry.json` fetch + SHA256 install into `$CONFIG_DIR/plugins` |
| `web/` | React source and the installable PWA (`public/manifest.webmanifest`, `public/sw.js`) |
| `.github/workflows` | CI, Docker Hub image publish, and tagged release binaries |

## Rules

| Rule | When |
| :--- | :--- |
| `.cursor/rules/project-core.mdc` | Every task |
| `.cursor/rules/versioning.mdc` | Every task — ask before bumps; show current `VERSION` |
| `.cursor/rules/go-backend.mdc` | `**/*.go` |
| `.cursor/rules/compat-api.mdc` | `internal/api/compat/**` |
| `.cursor/rules/web-ui.mdc` | `web/**` |
| `.cursor/rules/plugins.mdc` | Plugin SDK, examples, and host |

## Working agreement

- Prefer `make` targets over ad-hoc commands: `make start`, `make test`, `make build`.
- Product version is root `VERSION` only. Before any bump, tell the user the current version and ask; never assume.
- Local UI/API checks use `make start` (root `docker-compose.yml`) so media paths match production (`/data/TV`, `/data/Movies`). The dev container runs [Air](https://github.com/air-verse/air) (`.air.toml`) so Go edits rebuild/restart the API process in-place; Vite HMR covers `web/` edits. Restart the compose service only after Dockerfile/entrypoint/Air config changes. Stop with `make stop`.
- Run `go test -race ./...` for Go changes and `npm run typecheck` in `web/` for UI changes.
- Do not hand-edit `internal/webui/dist` or `web/dist`. Rebuild with `make build-web`.
- Do not commit databases, `.env`, API keys, or `./config/`.
- New permissions go in `internal/domain` and the built-in role seed, then the UI permission list.
- Compat routes must stay compatible with upstream Sonarr/Radarr/Prowlarr JSON. Read `docs/discovery/01`–`03` and the matching upstream controller on GitHub before changing a response, then update the discovery note.
- Keep search in-process. Do not add HTTP sync from a Prowlarr module into Sonarr or Radarr modules.
- Optional plugins install from [replacearr/replacearr-plugins](https://github.com/replacearr/replacearr-plugins) via the marketplace catalog URL in AppConfig / Plugins UI (`registry.json`); do not add that repo as a git submodule of Replacearr.
- Auth stays JWT (`Authorization: Bearer`, `replacearr_token` cookie) plus `*arr` keys (`X-Api-Key`, `?apikey=`). Key scopes are `all`, `sonarr`, `radarr`, `lidarr`, `readarr`, or `prowlarr`.
- Music (MusicBrainz) and Books/ebook+audiobook (OpenLibrary) are native modules (`docs/discovery/05` / `06`). Thin `/lidarr` and `/readarr` mounts are for migration only.

## UI navigation

The SPA uses accordion left-nav groups (`web/src/nav.ts`, `SideNav`) with `activeView` ids such as `series.library`, `settings.media`, and `system.logs`. Media libraries (TV / Movies / Music / Books) each expose Library, Library Import, Statistics, and Blocklist. Calendar and System are top-level. Settings categories live in the left nav (not an in-page submenu). Backup/Updates/Events/Logs/Changelog live under System.

## Still open from the architecture plan

- Raise Go and compat test coverage toward the routes in `docs/discovery/`.
- Full PWA support for the embedded SPA.
- Deeper Decision Engine (custom formats, RSS sync runners) beyond blocklist rejection on grab.

## Verify

```bash
make test-go             # go test -v -race ./...
make test-web            # tsc --noEmit
make build               # SPA + plugins + static binary
make start               # containerized API+UI with /data paths
```
