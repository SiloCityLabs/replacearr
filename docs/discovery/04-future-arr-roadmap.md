# 04 — Future *arr Ecosystem Roadmap & Plugin SDK Blueprint

## 1. Executive Summary

While Replacearr Phase 1 unified **Sonarr** (TV), **Radarr** (Movies), and **Prowlarr** (Indexers), **Music** and **Books** (ebook + audiobook) are now first-class native modules — see `05-music-module.md` and `06-books-module.md` (MusicBrainz / OpenLibrary; thin `/lidarr` and `/readarr` mounts only). **Subtitles** are also native (in-house OpenSubtitles pipeline under Series/Movies + Settings — no Bazarr process or `/bazarr` API). Remaining specialized applications:
1. ~~**Lidarr**~~ → native Music module (`05-music-module.md`)
2. ~~**Readarr**~~ → native Books module (`06-books-module.md`)
3. ~~**Bazarr**~~ → native Subtitles (`internal/subtitles`, Settings → Subtitles; embedded in Movie/Series detail)
4. **Whisparr** (Adult Video: Studios, Scenes, SceneFiles)

As evidenced in `Prowlarr/src/NzbDrone.Core/Applications/` (`Lidarr/`, `Readarr/`, `Whisparr/`) and `Prowlarr/src/NzbDrone.Core/Indexers/NewznabStandardCategory.cs` (categories `3000 Audio`, `6000 XXX`, `7000 Books`), Replacearr's indexer, decision, and download client engine already speaks the Torznab/Newznab search modes (`music-search`, `book-search`, `tv-search`, `movie-search`) required by all of these applications.

This document provides the architectural blueprint for adding remaining *arr capabilities into Replacearr—either as native domain modules or via Replacearr's `MediaModulePlugin` interface in `pkg/pluginsdk`.

---

## 2. Domain & Compatibility Blueprints per Application

### 2.1 Lidarr (Music Automation — `/lidarr/api/v1`)
* **Upstream Prowlarr Reference**: `Prowlarr/src/NzbDrone.Core/Applications/Lidarr/Lidarr.cs`, `LidarrV1Proxy.cs`, `LidarrSettings.cs`
* **API Version & Prefix**: Lidarr uses **`/api/v1`** (mounted in Replacearr at `/lidarr/api/v1/*`).
* **Entity Hierarchy**:
  ```
  Artist (mbId / ForeignArtistId)
   └── Album (ForeignAlbumId, AlbumType, ReleaseStatuses, Monitored)
        └── AlbumRelease (Mediums, Country, Disambiguation)
             └── Track (TrackNumber, AbsoluteTrackNumber, Duration, Explicit)
                  └── TrackFile (Path, Quality, MediaInfo, AudioTags)
  ```
* **Metadata Source**: **MusicBrainz** (via Lidarr Metadata API / MusicBrainz WS v2 + Cover Art Archive + Fanart.tv). Primary external identifier is `foreignArtistId` / `foreignAlbumId` (MusicBrainz UUID strings, e.g., `"b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d"`).
* **Profile Differences**:
  * In addition to `QualityProfile` (which ranks audio qualities: `MP3-192`, `MP3-320`, `MP3-VBR-V0`, `AAC-256`, `FLAC`, `FLAC 24bit`, `ALAC`, `WAV`), Lidarr introduces a **`MetadataProfile`** (`/api/v1/metadataprofile`) that filters which `PrimaryAlbumTypes` (`Album`, `EP`, `Single`, `Broadcast`, `Other`) and `SecondaryAlbumTypes` (`Studio`, `Compilation`, `Live`, `Remix`, `Soundtrack`) and `ReleaseStatuses` (`Official`, `Promotional`, `Bootleg`) are imported per artist.
* **Torznab / Newznab Search Modes**:
  * `t=music` with params `q`, `artist`, `album`, `track`, `label`, `year`, `genre` (`MusicSearchParam` in `IndexerCapabilities.cs`).
  * Categories: `3000` (`Audio`), `3010` (`Audio/MP3`), `3020` (`Audio/Video`), `3040` (`Audio/Lossless`), `3050` (`Audio/Other`).
* **Required Compatibility Endpoints (`/lidarr/api/v1`)**:
  * `/system/status` (`"appName": "Lidarr"`, `"version": "2.7.1.4410"`)
  * `/artist`, `/artist/{id}`, `/artist/lookup?term=...`
  * `/album`, `/album/{id}`, `/album/lookup?term=...`
  * `/track`, `/trackfile`
  * `/qualityprofile`, `/metadataprofile`, `/rootfolder`, `/tag`
  * `/calendar`, `/queue`, `/history`, `/command` (`ArtistSearch`, `AlbumSearch`, `RefreshArtist`, `RescanFolders`, `RssSync`)

---

### 2.2 Readarr (Books & Audiobooks — `/readarr/api/v1`)
* **Upstream Prowlarr Reference**: `Prowlarr/src/NzbDrone.Core/Applications/Readarr/Readarr.cs`, `ReadarrV1Proxy.cs`, `ReadarrSettings.cs`
* **API Version & Prefix**: Readarr uses **`/api/v1`** (mounted in Replacearr at `/readarr/api/v1/*`).
* **Entity Hierarchy**:
  ```
  Author (ForeignAuthorId)
   └── Book (ForeignBookId, TitleSlug, SeriesLinks, ReleaseDate, Monitored)
        └── Edition (ForeignEditionId, Isbn13, Asin, Format, PageCount, Monitored)
             └── BookFile (Path, Quality, Part, PartCount)
  ```
* **Metadata Source**: **Goodreads / OpenLibrary / Hardcover** metadata providers (`foreignAuthorId`, `foreignBookId`, `foreignEditionId`, `isbn13`, `asin`).
* **Profile & Media Type Nuances**:
  * Users frequently run two logical Readarr profiles: **eBooks** (`EPUB`, `MOBI`, `AZW3`, `PDF`, `CBZ`/`CBR` Comics) and **Audiobooks** (`M4B`, `MP3`, `FLAC`).
  * Because Replacearr supports `media_type` on `root_folders` (`ebook`, `audiobook`) and unified `quality_profiles`, a single Replacearr instance can manage both eBooks and Audiobooks without running two separate Readarr containers!
  * Also uses `MetadataProfile` (`/api/v1/metadataprofile`) to filter minimum popularity/pages and ignored languages.
* **Torznab / Newznab Search Modes**:
  * `t=book` with params `q`, `title`, `author`, `publisher`, `year`, `genre` (`BookSearchParam` in `IndexerCapabilities.cs`).
  * Categories: `7000` (`Books`), `7010` (`Books/Mags`), `7020` (`Books/EBook`), `7030` (`Books/Comics`), `7040` (`Books/Technical`), and `3030` (`Audio/Audiobook`).
* **Required Compatibility Endpoints (`/readarr/api/v1`)**:
  * `/system/status` (`"appName": "Readarr"`, `"version": "0.4.4.2635"`)
  * `/author`, `/author/{id}`, `/author/lookup?term=...`
  * `/book`, `/book/{id}`, `/book/lookup?term=...`, `/edition`
  * `/bookfile`, `/qualityprofile`, `/metadataprofile`, `/rootfolder`, `/tag`
  * `/calendar`, `/queue`, `/history`, `/command` (`AuthorSearch`, `BookSearch`, `RefreshAuthor`, `RescanFolders`, `RssSync`)

---

### 2.3 Native Subtitles (replaces Bazarr — no `/bazarr` API)
* **Product decision**: Fully **in-house**. There is no external Bazarr process, no `/bazarr` mount, and no work aimed at keeping a separate Bazarr container happy. OpenSubtitles.com is an upstream **catalog** (credentials in Settings), not a companion app.
* **UX**: Subtitles attach to existing video — shown on **Movie** and **Series** detail (per downloaded episode). Global languages / OpenSubtitles API key live under **Settings → Subtitles**. No top-level Subtitles/Bazarr nav tab.
* **Data**:
  * `episode_files` / `movie_files` — absolute `path`, `size`, `scene_name`, `release_group` (required for OSHash matching), linked via `episode_file_id` / `movie_file_id`.
  * `subtitle_profiles` — seeded Default profile; languages also mirrored in `AppConfig.subtitles`.
  * `subtitle_tracks` — downloaded / discovered sidecar rows.
* **Pipeline** (`internal/subtitles`):
  1. On CDH import, download monitor upserts the media file row and calls `OnMovieImported` / `OnEpisodeImported`.
  2. OSHash + OpenSubtitles REST (`/subtitles`, `POST /download`) → write `{videoBase}.{lang}.srt` beside the video.
  3. Native API: `GET/POST /api/v1/movies/{id}/subtitles…`, `…/episodes/{id}/subtitles…`.
* **Later (not required for v1)**: additional providers (Subdl, Addic7ed, …), ffprobe embedded-track inspection, ffsubsync/alass. Optional `SubtitleProviderPlugin` only when a second provider lands.

---

### 2.4 Whisparr (Adult Video Automation — `/whisparr/api/v3`)
* **Upstream Prowlarr Reference**: `Prowlarr/src/NzbDrone.Core/Applications/Whisparr/Whisparr.cs`, `WhisparrV3Proxy.cs`, `WhisparrSettings.cs`
* **API Version & Prefix**: Whisparr is a direct fork of Sonarr/Radarr v3 and exposes **`/api/v3`** (mounted in Replacearr at `/whisparr/api/v3/*`).
* **Entity Hierarchy**:
  * **Whisparr v2 (Sonarr-based)**: `Studio` (mapped as `Series`) -> `Year/Season` -> `Scene/Episode` (`tvdbId` mapped to **ThePornDB (TPDB)** / **StashDB** scene ID).
  * **Whisparr v3 (Radarr/Eros-based)**: Supports both **Scenes** and **Movies** (`Studio` -> `Movie`/`Scene` -> `MovieFile`), backed by **StashDB** and **ThePornDB** GraphQL metadata (`foreignId` / `stashId` / `tpdbId`).
* **Torznab / Newznab Search Modes**:
  * Categories: `6000` (`XXX`), `6010` (`XXX/DVD`), `6020` (`XXX/WMV`), `6030` (`XXX/XviD`), `6040` (`XXX/x264`), `6045` (`XXX/UHD`), `6050` (`XXX/Pack`), `6080` (`XXX/SD`), `6090` (`XXX/WEB-DL`).

---

## 3. Replacearr Extensibility: `MediaModulePlugin` (`pkg/pluginsdk`) vs. Native Domain Modules

Replacearr provides two clean pathways for adding these *arr capabilities: **Dynamic/Compiled Plugins (`pkg/pluginsdk`)** and **Native Domain Modules (`internal/domain` + `internal/db`)**.

### 3.1 Pathway A: The `pkg/pluginsdk` Interface Architecture
Developers can build self-contained media modules, indexers, download clients, or subtitle providers against `pkg/pluginsdk`:

```go
package pluginsdk

import (
	"context"
	"net/http"
)

// PluginMetadata identifies any Replacearr plugin.
type PluginMetadata struct {
	ID          string `json:"id"`          // e.g., "lidarr-music", "opensubtitles-extra"
	Name        string `json:"name"`        // e.g., "Lidarr Music Module"
	Version     string `json:"version"`     // e.g., "1.0.0"
	Author      string `json:"author"`
	Kind        string `json:"kind"`        // "media_module", "indexer", "download_client", "subtitle_provider"
	Description string `json:"description"`
}

// ReleaseCandidate represents a normalized indexer search result passed to a MediaModulePlugin.
type ReleaseCandidate struct {
	Guid        string
	Title       string
	IndexerID   int64
	IndexerName string
	Protocol    string // "torrent" | "usenet"
	Size        int64
	Seeders     int
	Peers       int
	DownloadURL string
	Categories  []int
	Attributes  map[string]string // Torznab/Newznab extended attributes (artist, album, author, booktitle, etc.)
}

// CompletedDownloadEvent is emitted by Replacearr's unified download monitor when a client finishes downloading.
type CompletedDownloadEvent struct {
	MediaType      string // "tv", "movie", "music", "book", "adult"
	DownloadID     string
	DownloadClient string
	Title          string
	OutputPath     string
	Size           int64
}

// MediaModulePlugin allows a new *arr domain (e.g., Lidarr, Readarr, Whisparr)
// to plug into Replacearr's shared SQLite database, Torznab search engine, and download queue.
type MediaModulePlugin interface {
	Metadata() PluginMetadata

	// MediaType returns the discriminator stored in root_folders, queue_items, and history_records
	// (e.g., "music", "book", "subtitle", "adult").
	MediaType() string

	// DefaultTorznabCategories returns the Newznab category IDs used by this module (e.g., [3000, 3010, 3040] for Lidarr).
	DefaultTorznabCategories() []int

	// Migrate runs module-specific SQLite schema migrations on startup.
	Migrate(ctx context.Context) error

	// RegisterCompatRoutes mounts the upstream *arr compatibility REST endpoints
	// (e.g., "/lidarr/api/v1" or "/readarr/api/v1") onto Replacearr's HTTP mux.
	RegisterCompatRoutes(mux *http.ServeMux)

	// ParseAndEvaluateRelease parses a release title and evaluates whether it matches a wanted item.
	ParseAndEvaluateRelease(ctx context.Context, candidate ReleaseCandidate) (accepted bool, score int, rejectReasons []string)

	// OnDownloadCompleted handles file import, renaming, and metadata tagging when a tracked download finishes.
	OnDownloadCompleted(ctx context.Context, event CompletedDownloadEvent) error
}
```

### 3.2 Pathway B: Native Domain Module Integration Checklist
When promoting a module into Replacearr's built-in core (`internal/`), follow this 5-step pattern:
1. **RBAC Permissions (`internal/domain/models.go`)**:
   Add module permission constants (e.g., `PermMusicRead = "music:read"`, `PermMusicWrite = "music:write"`, `PermBooksRead = "books:read"`) and include them in the built-in `admin`, `manager`, and `integration` roles in `internal/db/sqlite.go`. Subtitles reuse `tv:*` / `movies:*` (no separate Bazarr section).
2. **SQLite Migration (`internal/db/migrations/00N_<module>.sql`)**:
   Create the domain tables (e.g., `artists`, `albums`, `tracks`, `track_files` for Lidarr; `authors`, `books`, `book_files` for Readarr). Subtitle tables (`episode_files`, `movie_files`, `subtitle_profiles`, `subtitle_tracks`) are already native in `007_media_files_subtitles.sql`. No changes are needed for `tags`, `root_folders`, `quality_profiles`, `indexers`, `download_clients`, `queue_items`, or `history_records`, as those tables already use a `media_type` discriminator (`'tv'`, `'movie'`, `'music'`, `'book'`, `'adult'`).
3. **Torznab Query Builder**:
   Dispatch searches using the existing `NewznabRequest` parameters (`t=music` with `artist`/`album` or `t=book` with `author`/`title`) against Replacearr's in-memory Cardigann and Plugin indexers.
4. **Compatibility Router**:
   Mount the compatibility handler group at `/<app>/api/v1` (or `/<app>/api/v3` for Whisparr), reusing the shared middleware for `X-Api-Key` / `?apikey=` authentication and shared handlers for `/system/status`, `/qualityprofile`, `/rootfolder`, `/tag`, `/downloadclient`, `/indexer`, `/queue`, and `/history`. Native subtitles have no `/bazarr` compat mount.
5. **Prowlarr Virtual Application Registration**:
   Expose the module in `GET /prowlarr/api/v1/applications` as a built-in `fullSync` application so users and dashboards see unified indexer coverage across TV, Movies, Music, and Books.
