# Replacearr

**Replacearr** is a fast, single-binary, Go-powered replacement for **Sonarr**, **Radarr**, and **Prowlarr** — one container, one SQLite database, one web UI.

- Unified TV, movie, indexer, and download-client management
- Drop-in compatible **Sonarr v3**, **Radarr v3**, and **Prowlarr v1** APIs, so Overseerr, Jellyseerr, Bazarr, Homepage, LunaSea, nzb360 and friends work unchanged
- Multi-user accounts with role-based access control (RBAC) and scoped API keys
- Native Cardigann YAML indexer definitions (Prowlarr/Jackett compatible)
- Out-of-process Go plugin SDK for custom indexers, download clients, and notifications
- Built-in FlareSolverr integration for Cloudflare-protected indexers
- LinuxServer-style `PUID` / `PGID` / `TZ` support

Source code, issues, and full documentation: **https://github.com/replacearr/replacearr**

**Official image:** [`ldrrp/replacearr` on Docker Hub](https://hub.docker.com/r/ldrrp/replacearr)

---

## Quick Start

### Docker Compose (recommended)

```yaml
services:
  replacearr:
    image: ldrrp/replacearr:latest
    container_name: replacearr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Etc/UTC
      - REPLACEARR_ADMIN_USER=admin
      - REPLACEARR_ADMIN_PASS=change-me-now
      - REPLACEARR_FLARESOLVERR_URL=http://flaresolverr:8191/
    volumes:
      - ./config:/config
      - /path/to/media:/data
    ports:
      - 8989:8989
    depends_on:
      - flaresolverr
    restart: unless-stopped

  # Optional: only needed for Cloudflare-protected indexers
  flaresolverr:
    image: ghcr.io/flaresolverr/flaresolverr:latest
    container_name: flaresolverr
    environment:
      - LOG_LEVEL=info
      - TZ=Etc/UTC
    tmpfs:
      - /tmp:size=256M,mode=1777
    restart: unless-stopped
```

```bash
docker compose up -d
```

### Docker CLI

```bash
docker run -d \
  --name=replacearr \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Etc/UTC \
  -e REPLACEARR_ADMIN_USER=admin \
  -e REPLACEARR_ADMIN_PASS=change-me-now \
  -p 8989:8989 \
  -v /path/to/config:/config \
  -v /path/to/media:/data \
  --restart unless-stopped \
  ldrrp/replacearr:latest
```

Then open **http://\<your-host\>:8989** and sign in with the admin credentials you set.

---

## First Run

On the very first start (when the database contains no users), Replacearr:

1. Creates the admin account from `REPLACEARR_ADMIN_USER` / `REPLACEARR_ADMIN_PASS`.
2. Creates a master API key with full access and writes it to **`/config/api_key.txt`**. If `REPLACEARR_API_KEY` is set, that value is used instead.
3. Generates a JWT signing secret and stores it at **`/config/jwt_secret.key`** (unless `REPLACEARR_JWT_SECRET` is set).
4. Seeds a FlareSolverr indexer proxy pointing at `REPLACEARR_FLARESOLVERR_URL`.
5. Copies bundled plugins into `/config/plugins`.

To read the generated master API key:

```bash
docker exec replacearr cat /config/api_key.txt
```

> **Important:** the admin credentials, API key, and FlareSolverr URL variables are only applied on first startup. After that, change them from the web UI. Updating the environment variables later has no effect on an existing database.

---

## Environment Variables

### Container

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PUID` | `1000` | User ID the application runs as. Match it to the owner of your media files. |
| `PGID` | `1000` | Group ID the application runs as. |
| `TZ` | `UTC` | Timezone, e.g. `America/New_York`. Use a [TZ database name](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones). |

### Server

| Variable | Default | Description |
| :--- | :--- | :--- |
| `REPLACEARR_HOST` | `0.0.0.0` | HTTP bind address. |
| `REPLACEARR_PORT` | `8989` | HTTP listen port. If you change it, update your port mapping. The built-in health check always probes port `8989`. |
| `REPLACEARR_ENV` | `production` | Runtime environment label. |

### Paths

| Variable | Default | Description |
| :--- | :--- | :--- |
| `REPLACEARR_CONFIG_DIR` | `/config` | Base directory for the database, secrets, plugins, and definitions. |
| `REPLACEARR_DB_PATH` | `/config/replacearr.db` | SQLite database file. |
| `REPLACEARR_PLUGIN_DIR` | `/config/plugins` | Directory scanned for executable Go plugins. |
| `REPLACEARR_DEFINITIONS_DIR` | `/config/definitions` | Directory scanned for Cardigann YAML indexer definitions. |

### Authentication (first run only)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `REPLACEARR_ADMIN_USER` | `admin` | Username of the initial admin account. |
| `REPLACEARR_ADMIN_PASS` | `replacearr` | Password of the initial admin account. **Change this.** |
| `REPLACEARR_API_KEY` | *(auto-generated)* | Fixed master API key, useful for automation. Generated as 32 hex characters if unset. |
| `REPLACEARR_JWT_SECRET` | *(auto-generated)* | Secret used to sign login tokens. If unset, it is generated once and persisted to `/config/jwt_secret.key`. |

### Integrations

| Variable | Default | Description |
| :--- | :--- | :--- |
| `REPLACEARR_FLARESOLVERR_URL` | `http://localhost:8191/` | FlareSolverr endpoint seeded as the default indexer proxy on first run. Inside Compose, use the service name, e.g. `http://flaresolverr:8191/`. |

---

## Volumes

| Path | Description |
| :--- | :--- |
| `/config` | Persistent application data: SQLite database, JWT secret, master API key, plugins, and Cardigann definitions. **Back this up.** |
| `/data` | Media library and downloads. The container creates `/data/tv`, `/data/movies`, and `/data/downloads` if they don't exist. |

Layout of `/config`:

```text
/config
├── replacearr.db       # SQLite database
├── jwt_secret.key      # JWT signing secret
├── api_key.txt         # Master API key (created on first run)
├── plugins/            # Executable Go plugins
└── definitions/        # Cardigann YAML indexer definitions
```

### Permissions

On startup the container runs `chown -R PUID:PGID` on `/config`. It does **not** change ownership of `/data`, so make sure your media and download folders are writable by the `PUID`/`PGID` you configure:

```bash
id your_user                                  # find your UID/GID
sudo chown -R 1000:1000 /path/to/media        # if needed
```

### Single `/data` mount for hardlinks

Mount one parent folder containing both your library and downloads at `/data` (rather than separate mounts for each). That way your download client and Replacearr see the same filesystem, so imports can use hardlinks and atomic moves instead of slow copies.

```text
/path/to/media
├── downloads/
├── movies/
└── tv/
```

Use the same `/data/...` paths inside your download client's container.

---

## Ports

| Port | Description |
| :--- | :--- |
| `8989` | Web UI, native API, and Sonarr / Radarr / Prowlarr compatibility APIs. |

---

## Connecting *arr-Compatible Apps

A single Replacearr instance can act as Sonarr, Radarr, and Prowlarr at the same time:

| Pretend to be | URL to enter in the client |
| :--- | :--- |
| Sonarr v3 | `http://replacearr:8989/sonarr` |
| Radarr v3 | `http://replacearr:8989/radarr` |
| Prowlarr v1 | `http://replacearr:8989/prowlarr` |

Root-level `/api/v3` and `/api/v1` routes are also available for clients that cannot use a URL prefix.

For the API key, use either the master key from `/config/api_key.txt`, or (better) create a dedicated key under **Users & RBAC → API Keys** with the `integration` role and a scope of `sonarr`, `radarr`, `prowlarr`, or `all`.

Authentication accepts the `X-Api-Key` header, the `?apikey=` query parameter, or `Authorization: Bearer <token>`.

---

## Plugins and Indexer Definitions

- **Plugins:** drop executable Go plugin binaries into `/config/plugins` and make them executable (`chmod +x`). Bundled plugins are copied there on startup only if a file with the same name doesn't already exist, so your own versions are never overwritten. See the [Plugin SDK docs](https://github.com/replacearr/replacearr#go-plugin-sdk-pkgpluginsdk).
- **Cardigann definitions:** place Prowlarr/Jackett-compatible `.yml` definitions in `/config/definitions`, e.g. from [Prowlarr/Indexers](https://github.com/Prowlarr/Indexers).

---

## Health Check

The image has a built-in Docker `HEALTHCHECK` that polls an unauthenticated endpoint every 30 seconds:

```bash
curl -fsS http://localhost:8989/api/v1/system/health
```

`docker ps` reports the container as `healthy` once the server is up.

---

## Reverse Proxy

Replacearr serves everything from the root path, so proxy a dedicated (sub)domain to port `8989`. Example for Nginx:

```nginx
server {
    listen 443 ssl;
    server_name replacearr.example.com;

    location / {
        proxy_pass http://replacearr:8989;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Updating

```bash
docker compose pull replacearr
docker compose up -d replacearr
```

All state lives in `/config`, so recreating the container is safe. Back up `/config` before major upgrades.

---

## Building the Image Yourself

From the root of the repository:

```bash
docker build -f docker/Dockerfile -t ldrrp/replacearr:latest .
```

Build arguments:

| Argument | Default | Description |
| :--- | :--- | :--- |
| `VERSION` | `0.1.0` | Version string embedded in the binary. |
| `COMMIT` | `prod` | Git commit embedded in the binary. |

Or use the bundled Compose stack, which builds locally and starts FlareSolverr:

```bash
docker compose -f docker/docker-compose.prod.yml up -d --build
```

---

## Troubleshooting

- **Can't log in:** the admin variables only apply to a fresh database. If you changed them after the first run, either use the original credentials or stop the container, move `/config/replacearr.db` aside, and start again (this resets all data).
- **Import or permission errors:** check that `PUID`/`PGID` match the owner of your `/data` folders.
- **FlareSolverr not reachable:** from inside Compose, use the service name (`http://flaresolverr:8191/`), not `localhost`. You can update the URL later under the indexer proxy settings in the UI.
- **View logs:** `docker logs -f replacearr`

---

## License

MIT
