# 04 — Future *arr Ecosystem Roadmap & Plugin SDK Blueprint

## 1. Executive Summary

While Replacearr Phase 1 unifies **Sonarr** (TV), **Radarr** (Movies), and **Prowlarr** (Indexers) into a single binary and SQLite database, the broader *arr media automation ecosystem includes four additional specialized applications:
1. **Lidarr** (Music: Artists, Albums, Tracks, TrackFiles)
2. **Readarr** (Books & Audiobooks: Authors, Books, Editions, BookFiles)
3. **Bazarr** (Subtitles: Subtitle Providers, Language Profiles, Embedded Track Extraction & Sync)
4. **Whisparr** (Adult Video: Studios, Scenes, SceneFiles)

As evidenced in `.tmp/Prowlarr/src/NzbDrone.Core/Applications/` (`Lidarr/`, `Readarr/`, `Whisparr/`) and `.tmp/Prowlarr/src/NzbDrone.Core/Indexers/NewznabStandardCategory.cs` (categories `3000 Audio`, `6000 XXX`, `7000 Books`), Replacearr's indexer, decision, and download client engine already speaks the Torznab/Newznab search modes (`music-search`, `book-search`, `tv-search`, `movie-search`) required by all of these applications.

This document provides the architectural blueprint for adding **Lidarr**, **Readarr**, **Bazarr**, and **Whisparr** into Replacearr—either as native domain modules or via Replacearr's `MediaModulePlugin` interface in `pkg/pluginsdk`.

---

## 2. Domain & Compatibility Blueprints per Application

### 2.1 Lidarr (Music Automation — `/lidarr/api/v1`)
* **Upstream Prowlarr Reference**: `.tmp/Prowlarr/src/NzbDrone.Core/Applications/Lidarr/Lidarr.cs`, `LidarrV1Proxy.cs`, `LidarrSettings.cs`
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
* **Upstream Prowlarr Reference**: `.tmp/Prowlarr/src/NzbDrone.Core/Applications/Readarr/Readarr.cs`, `ReadarrV1Proxy.cs`, `ReadarrSettings.cs`
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

### 2.3 Bazarr (Subtitle Management & Sync — `/bazarr/api`)
* **Architectural Role**: Unlike Lidarr/Readarr (which are forks of `NzbDrone.Core`), **Bazarr** is a Python companion daemon that polls Sonarr (`/api/v3/series`, `/api/v3/episode`, `/api/v3/episodefile`) and Radarr (`/api/v3/movie`, `/api/v3/moviefile`) to discover video files on disk, inspect their embedded audio/subtitle streams, and download missing `.srt`/`.ass` subtitles from external subtitle providers.
* **Why Native Integration in Replacearr is Transformative**:
  * In a traditional stack, Bazarr must poll Sonarr and Radarr over HTTP or wait for SignalR events to learn when an `EpisodeFile` or `MovieFile` is imported.
  * Inside Replacearr, the Subtitle Module subscribes directly to the internal `EpisodeImported` and `MovieImported` domain events! The moment `CompletedDownloadService` imports a file into `/data/tv` or `/data/movies`, Replacearr's subtitle pipeline already knows the exact `path`, `sceneName`, `releaseGroup`, `imdbId`, `tvdbId`, `tmdbId`, and `mediaInfo` audio/subtitle tracks.
* **Core Subsystems Required**:
  1. **Language Profiles (`subtitle_profiles`)**: Target subtitle languages (`en`, `es`, `fr`, etc.), `forced` vs. `normal` vs. `hi` (Hearing Impaired) preferences, and minimum match score thresholds.
  2. **Embedded Track Extraction & Inspection**: `ffprobe` / MKV header parser (`matroska`) to detect existing embedded text (`subrip`, `ass`, `webvtt`) and bitmap (`hdmv_pgs_subtitle`, `dvd_subtitle`) tracks, optionally extracting embedded subtitles to sidecar `.srt` files.
  3. **Subtitle Provider Plugin Interface (`SubtitleProviderPlugin`)**:
     * Providers: OpenSubtitles.com (REST API + file hash matching), Subdl, Podnapisi, Addic7ed, Gestdown, AnimeTosho, Whisper/AI transcription.
     * Scoring matches by video file hash, `sceneName` (`Series` + `Season` + `Episode` + `ReleaseGroup` + `Source` + `Resolution` + `VideoCodec`).
  4. **Audio-to-Subtitle Synchronization**: Optional `ffsubsync` / `alass` integration to align subtitle timestamps to the video's audio stream.

---

### 2.4 Whisparr (Adult Video Automation — `/whisparr/api/v3`)
* **Upstream Prowlarr Reference**: `.tmp/Prowlarr/src/NzbDrone.Core/Applications/Whisparr/Whisparr.cs`, `WhisparrV3Proxy.cs`, `WhisparrSettings.cs`
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
	ID          string `json:"id"`          // e.g., "lidarr-music", "bazarr-subtitles"
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

// MediaModulePlugin allows a new *arr domain (e.g., Lidarr, Readarr, Whisparr, Bazarr)
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
   Add module permission constants (e.g., `PermMusicRead = "music:read"`, `PermMusicWrite = "music:write"`, `PermBooksRead = "books:read"`, `PermSubtitlesRead = "subtitles:read"`) and include them in the built-in `admin`, `manager`, and `integration` roles in `internal/db/sqlite.go`.
2. **SQLite Migration (`internal/db/migrations/002_<module>.sql`)**:
   Create the domain tables (e.g., `artists`, `albums`, `tracks`, `track_files` for Lidarr; `authors`, `books`, `book_files` for Readarr; `subtitle_tracks` for Bazarr). No changes are needed for `tags`, `root_folders`, `quality_profiles`, `indexers`, `download_clients`, `queue_items`, or `history_records`, as those tables already use a `media_type` discriminator (`'tv'`, `'movie'`, `'music'`, `'book'`, `'adult'`).
3. **Torznab Query Builder**:
   Dispatch searches using the existing `NewznabRequest` parameters (`t=music` with `artist`/`album` or `t=book` with `author`/`title`) against Replacearr's in-memory Cardigann and Plugin indexers.
4. **Compatibility Router**:
   Mount the compatibility handler group at `/<app>/api/v1` (or `/<app>/api/v3` for Whisparr), reusing the shared middleware for `X-Api-Key` / `?apikey=` authentication and shared handlers for `/system/status`, `/qualityprofile`, `/rootfolder`, `/tag`, `/downloadclient`, `/indexer`, `/queue`, and `/history`.
5. **Prowlarr Virtual Application Registration**:
   Expose the module in `GET /prowlarr/api/v1/applications` as a built-in `fullSync` application so users and dashboards see unified indexer coverage across TV, Movies, Music, Books, and Subtitles.
