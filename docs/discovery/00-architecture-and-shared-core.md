# 00 — Architecture & Shared Core (`NzbDrone.Core` to Replacearr Unified Core)

## 1. Executive Overview

Upstream **Sonarr**, **Radarr**, and **Prowlarr** are separate C#/.NET forks derived from the same `NzbDrone` codebase. Each repository maintains its own copy of `src/NzbDrone.Core`, its own SQLite database (`sonarr.db`, `radarr.db`, `prowlarr.db`), its own background scheduler, and its own HTTP server. Because they run as isolated processes with separate databases, users must run 3+ daemons and configure Prowlarr's `ApplicationService` to continuously push and sync indexer configurations over HTTP into Sonarr and Radarr.

**Replacearr** consolidates the shared `NzbDrone.Core` primitives—Quality Profiles, Custom Formats, Release Parsing, Decision Engine, Download Clients, Completed Download Handling, Root Folders, and Tags—into a single Go binary backed by a single unified SQLite database (`internal/db`). It exposes native `/api/v1/*` endpoints alongside 100% wire-compatible virtual routers for Sonarr (`/api/v3` and `/sonarr/api/v3`), Radarr (`/radarr/api/v3`), and Prowlarr (`/prowlarr/api/v1`), eliminating HTTP self-syncing entirely.

---

## 2. How `NzbDrone.Core` is Shared Across Sonarr, Radarr, and Prowlarr

Inspection of `.tmp/Sonarr/src/NzbDrone.Core`, `.tmp/Radarr/src/NzbDrone.Core`, and `.tmp/Prowlarr/src/NzbDrone.Core` reveals that ~75% of the codebase is structurally identical across the three applications, differing only in the domain entity (`Series`/`Episode` vs. `Movie` vs. `Indexer`-only).

| Subsystem | Sonarr (`src/NzbDrone.Core`) | Radarr (`src/NzbDrone.Core`) | Prowlarr (`src/NzbDrone.Core`) | Replacearr (`internal/`) |
| :--- | :--- | :--- | :--- | :--- |
| **Provider Framework** | `ThingiProvider/IProvider.cs`, `ProviderFactory.cs`, `ProviderDefinition.cs` | Identical (`ThingiProvider/*`) | Identical (`ThingiProvider/*`) | Unified `domain.Indexer`, `domain.DownloadClient`, `pkg/pluginsdk` |
| **Quality & Profiles** | `Qualities/Quality.cs`, `Profiles/Qualities/QualityProfile.cs` | Identical (`Qualities/*`, `Profiles/Qualities/*`) | *N/A* (Uses `Profiles/AppSyncProfile.cs`) | `quality_profiles` table (`internal/db/sqlite.go`) |
| **Custom Formats** | `CustomFormats/CustomFormat.cs`, `CustomFormatCalculationService.cs` | Identical (`CustomFormats/*`, adapted for `Movie`) | *N/A* | Unified Custom Format & Score Evaluator |
| **Release Parsing** | `Parser/Parser.cs` (`ReportTitleRegex`), `QualityParser.cs`, `LanguageParser.cs` | `Parser/Parser.cs` (`ReportMovieTitleRegex`), `QualityParser.cs` | `Parser/ParseUtil.cs`, `Model/ReleaseInfo.cs`, `Model/TorrentInfo.cs` | Shared Release & Quality Parser |
| **Decision Engine** | `DecisionEngine/DownloadDecisionMaker.cs`, `Specifications/*` (54 files) | `DecisionEngine/DownloadDecisionMaker.cs`, `Specifications/*` | *N/A* | Unified Decision Engine Pipeline |
| **Download Clients** | `Download/IDownloadClient.cs`, `TorrentClientBase.cs`, `UsenetClientBase.cs`, `Clients/*` | Identical (`Download/Clients/*`) | Identical (`Download/Clients/*`) | `download_clients` table & Client Registry |
| **Download Tracking & Import** | `Download/TrackedDownloads/TrackedDownloadService.cs`, `CompletedDownloadService.cs` | Identical (`TrackedDownloadService.cs`, `CompletedDownloadService.cs`) | `Download/DownloadService.cs` (grab-only) | `queue_items` & `history_records` (`media_type` discriminator) |
| **Root Folders** | `RootFolders/RootFolderService.cs`, `RootFolder.cs` | Identical (`RootFolders/*`) | *N/A* | `root_folders` table with `media_type` (`tv`, `movie`, `all`) |
| **Tags** | `Tags/TagService.cs`, `Tag.cs`, `TagDetails.cs` | Identical (`Tags/*`) | Identical (`Tags/*`) | `tags` table shared across all entities |

---

## 3. Deep Dive into Core `NzbDrone.Core` Subsystems

### 3.1 Quality Definitions & Quality Profiles
* **Upstream Source**:
  * `.tmp/Sonarr/src/NzbDrone.Core/Qualities/Quality.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Qualities/QualityModel.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Profiles/Qualities/QualityProfile.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Profiles/Qualities/QualityProfileQualityItem.cs`
* **Mechanics**:
  * `Quality` defines static definitions with integer IDs (`0` = `Unknown`, `1` = `SDTV`, `2` = `DVD`, `3` = `WEBDL-1080p`, `4` = `HDTV-720p`, `5` = `WEBDL-720p`, `6` = `Bluray-720p`, `7` = `Bluray-1080p`, `8` = `WEBDL-480p`, `9` = `HDTV-1080p`, `10` = `Raw-HD`, `12` = `WEBRip-480p`, `13` = `Bluray-480p`, `14` = `WEBRip-720p`, `15` = `WEBRip-1080p`, `16` = `HDTV-2160p`, `17` = `WEBRip-2160p`, `18` = `WEBDL-2160p`, `19` = `Bluray-2160p`, `20` = `Bluray-1080p Remux`, `21` = `Bluray-2160p Remux`, `22` = `Bluray-576p`), each with a `QualitySource` (`television`, `web`, `webRip`, `dvd`, `bluray`, `blurayRaw`) and `Resolution` (`480`, `576`, `720`, `1080`, `2160`).
  * `QualityModel` wraps a `Quality` alongside a `Revision` (`version`, `real`, `isRepack`) to track `PROPER` and `REPACK` releases.
  * `QualityProfile` orders `Items` (`List<QualityProfileQualityItem>`) from lowest to highest preference. Items can be standalone qualities or nested quality groups (e.g., `WEB 1080p` grouping `WEBDL-1080p` and `WEBRip-1080p`).
  * `QualityProfile.GetIndex(int id, bool respectGroupOrder)` resolves the rank of a candidate quality against `Cutoff` when `UpgradeAllowed` is `true`.

### 3.2 Custom Formats & Scoring
* **Upstream Source**:
  * `.tmp/Sonarr/src/NzbDrone.Core/CustomFormats/CustomFormat.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/CustomFormats/CustomFormatCalculationService.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/CustomFormats/Specifications/*` (`ReleaseTitleSpecification.cs`, `ReleaseGroupSpecification.cs`, `SourceSpecification.cs`, `ResolutionSpecification.cs`, `LanguageSpecification.cs`, `SizeSpecification.cs`, `IndexerFlagSpecification.cs`, `ReleaseTypeSpecification.cs`)
* **Mechanics**:
  * Each `CustomFormat` contains a list of `ICustomFormatSpecification` rules.
  * In `CustomFormatCalculationService.ParseCustomFormat(CustomFormatInput input, List<CustomFormat> allCustomFormats)`, specifications are grouped by their concrete `.GetType()` into `SpecificationMatchesGroup`. A group matches if:
    * All `Required == true` specifications in the group match, **AND**
    * If no required specs exist in that group, at least one optional (`Required == false`) specification matches (`Matches.Any(m => m.Value)`), with `Negate == true` inverting individual spec evaluations.
  * A `CustomFormat` matches when **every** `SpecificationMatchesGroup` evaluates `DidMatch == true`.
  * `QualityProfile.CalculateCustomFormatScore(List<CustomFormat> formats)` sums the `ProfileFormatItem.Score` for all matched formats. The candidate release must have a score `>= MinFormatScore`, and upgrades require improving either the `QualityIndex` or increasing the Custom Format score by at least `MinUpgradeFormatScore` up to `CutoffFormatScore`.

### 3.3 Release Parsing (`Parser.cs`, `QualityParser.cs`, `LanguageParser.cs`)
* **Upstream Source**:
  * `.tmp/Sonarr/src/NzbDrone.Core/Parser/Parser.cs`
  * `.tmp/Radarr/src/NzbDrone.Core/Parser/Parser.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Parser/QualityParser.cs`
* **Mechanics**:
  * **Sonarr (`ReportTitleRegex`)**: Evaluates ~45 ordered compiled regular expressions to extract `title`, `season`, `episode` (multi-episode, split episode, daily `airyear`-`airmonth`-`airday`, anime `[SubGroup]` + `absoluteepisode`, full season packs, and partial season packs) into `ParsedEpisodeInfo`.
  * **Radarr (`ReportMovieTitleRegex`)**: Evaluates movie-specific regexes to extract `title`, `year`, `edition` (`Director's Cut`, `Extended`, `IMAX`, `Remastered`, `Unrated`, `Open Matte`), `imdbid`, and `tmdbid` into `ParsedMovieInfo`.
  * **Shared Pre/Post-Processing**: Both strip hashed release names (`RejectHashedReleasesRegex`), detect reversed titles (`p027`/`p0801`), normalize umlauts/diacritics/articles (`CleanSeriesTitle` / `CleanMovieTitle`), and invoke `QualityParser.ParseQuality(title)` and `LanguageParser.ParseLanguages(title)` plus `ReleaseGroupParser.ParseReleaseGroup(title)`.

### 3.4 Decision Engine (`DownloadDecisionMaker.cs` & Specifications)
* **Upstream Source**:
  * `.tmp/Sonarr/src/NzbDrone.Core/DecisionEngine/DownloadDecisionMaker.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/DecisionEngine/DownloadDecisionPriorizationService.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/DecisionEngine/DownloadDecisionComparer.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/DecisionEngine/Specifications/*`
* **Mechanics**:
  * `DownloadDecisionMaker.GetDecisions` parses each `ReleaseInfo`, maps it to a domain target (`RemoteEpisode` or `RemoteMovie`), augments it with scene mappings and Custom Formats, and runs all `IDownloadDecisionEngineSpecification` implementations grouped by `SpecificationPriority` (`Default`, `Parsing`, `Database`, `Disk`).
  * Key specifications include:
    * `QualityAllowedByProfileSpecification` (is quality enabled in profile?)
    * `CustomFormatAllowedByProfileSpecification` (is CF score `>= MinFormatScore`?)
    * `UpgradableSpecification` / `UpgradeDiskSpecification` / `UpgradeAllowedSpecification` (does release upgrade existing disk file?)
    * `AcceptableSizeSpecification` & `MaximumSizeSpecification` (within `QualityDefinition` min/max MB-per-minute bounds?)
    * `MinimumAgeSpecification`, `RetentionSpecification`, `TorrentSeedingSpecification` (min seeders), `BlocklistSpecification`, `QueueSpecification`, `ReleaseRestrictionsSpecification` (required/ignored terms).
  * If any specification returns `DownloadSpecDecision.Reject(reason, message)`, a `DownloadRejection` (`Permanent` or `Temporary`) is attached.
  * Accepted decisions are ranked by `DownloadDecisionComparer` (Protocol preference -> Quality index -> Custom Format score -> Proper/Repack revision -> Seeders/Peers -> Age -> Size).

### 3.5 Download Clients & Completed Download Handling
* **Upstream Source**:
  * `.tmp/Sonarr/src/NzbDrone.Core/Download/IDownloadClient.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Download/TorrentClientBase.cs` & `UsenetClientBase.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Download/TrackedDownloads/TrackedDownloadService.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Download/CompletedDownloadService.cs`
* **Mechanics**:
  * `IDownloadClient` abstracts Torrent (`qBittorrent`, `Transmission`, `Deluge`, `rTorrent`, `uTorrent`, `Aria2`, `Flood`, `RQbit`) and Usenet (`SABnzbd`, `NZBGet`, `Pneumatic`) clients via `Download(RemoteEpisode/RemoteMovie, IIndexer)` returning a `downloadClientId` string (typically infohash or NZB ID), `GetItems()` returning `IEnumerable<DownloadClientItem>`, and `RemoveItem()`.
  * `DownloadMonitoringService` periodically polls enabled download clients, passing each `DownloadClientItem` into `TrackedDownloadService.TrackDownload()`.
  * `TrackedDownloadService` correlates the `DownloadId` against `History` (`Grabbed` event) or parses the item title to attach `RemoteEpisode`/`RemoteMovie`, maintaining a state machine (`TrackedDownloadState`: `Downloading` -> `ImportPending` -> `Importing` -> `Imported` / `ImportBlocked` / `FailedPending` / `Failed` / `Ignored`).
  * When `DownloadItem.Status == DownloadItemStatus.Completed`, `CompletedDownloadService.Check()` verifies path accessibility (applying `RemotePathMapping` if configured) and transitions the item to `ImportPending`, after which `CompletedDownloadService.Import()` invokes `DownloadedEpisodesImportService` / `DownloadedMoviesImportService` to hardlink/copy/move the media file into the `RootFolder`, create an `EpisodeFile`/`MovieFile` record, and emit `DownloadCompletedEvent`.

### 3.6 Root Folders & Tags
* **Upstream Source**:
  * `.tmp/Sonarr/src/NzbDrone.Core/RootFolders/RootFolderService.cs`
  * `.tmp/Sonarr/src/NzbDrone.Core/Tags/TagService.cs`
* **Mechanics**:
  * `RootFolderService` validates disk paths, calculates `FreeSpace` and `TotalSpace`, and scans immediate subdirectories to return `UnmappedFolders` (folders on disk not yet linked to a `Series` or `Movie` in the database).
  * `TagService` manages lowercase alphanumeric + hyphen labels (`^[a-z0-9-]+$`) and computes `TagDetails` (cross-referencing which Series/Movies, Notifications, Release Profiles, Indexers, Download Clients, and AutoTagging rules use each Tag ID).

---

## 4. How Replacearr Unifies the Core & Eliminates HTTP Self-Syncing

### 4.1 The Problem with Upstream Prowlarr Application Sync
In upstream `.tmp/Prowlarr/src/NzbDrone.Core/Applications/ApplicationService.cs` and `.tmp/Prowlarr/src/NzbDrone.Core/Applications/Sonarr/Sonarr.cs` (and `Radarr.cs`):
1. Every time an indexer is added, updated, or deleted in Prowlarr (`ProviderAddedEvent<IIndexer>`, `ProviderUpdatedEvent<IIndexer>`, `ApplicationIndexerSyncCommand`), `ApplicationService.SyncIndexers()` iterates over all configured target applications (`Sonarr`, `Radarr`, `Lidarr`, `Readarr`, `Whisparr`).
2. For each app, Prowlarr makes HTTP calls (`GET /api/v3/indexer/schema`, `GET /api/v3/indexer`, `POST /api/v3/indexer`, `PUT /api/v3/indexer/{id}`, `DELETE /api/v3/indexer/{id}`) using `SonarrV3Proxy` / `RadarrV3Proxy`.
3. Prowlarr synthesizes a proxy `Torznab` or `Newznab` indexer inside Sonarr/Radarr with `Name: "{indexer.Name} (Prowlarr)"`, `baseUrl: "{ProwlarrUrl}/{indexer.Id}/"`, `apiPath: "/api"`, and `apiKey: "{ProwlarrApiKey}"`, and tracks the foreign key pair `(IndexerId, RemoteIndexerId)` in Prowlarr's `AppIndexerMap` table.
4. When Sonarr/Radarr performs an RSS sync or search, it issues an HTTP request to Prowlarr's `NewznabController` (`/{id}/api?t=tvsearch...`), which translates the request, queries the upstream tracker, rewrites download URLs to proxy links (`/{id}/download?link=...`), serializes the results to Torznab XML, sends them back over HTTP to Sonarr/Radarr, which then parses the XML back into `ReleaseInfo` objects!

```
UPSTREAM ARCHITECTURE (3 Processes, 3 SQLite DBs, HTTP Self-Sync & XML Round-Trip)
==================================================================================
[Prowlarr DB] ──(HTTP POST /api/v3/indexer)──► [Sonarr DB] ("YTS (Prowlarr)")
[Prowlarr DB] ──(HTTP POST /api/v3/indexer)──► [Radarr DB] ("YTS (Prowlarr)")
      ▲                                              │
      └──────(HTTP GET /{id}/api?t=tvsearch XML)─────┘


REPLACEARR ARCHITECTURE (1 Process, 1 Unified SQLite DB, Zero-Copy In-Memory Search)
====================================================================================
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              Replacearr Single Binary                            │
│                                                                                  │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌──────────────────────────┐  │
│  │ Sonarr v3 Compat API│  │ Radarr v3 Compat API│  │ Prowlarr v1 & Torznab API│  │
│  │  (/sonarr/api/v3)   │  │  (/radarr/api/v3)   │  │    (/prowlarr/api/v1)    │  │
│  └──────────┬──────────┘  └──────────┬──────────┘  └────────────┬─────────────┘  │
│             │                        │                          │                │
│             └────────────────────────┼──────────────────────────┘                │
│                                      ▼                                           │
│             ┌────────────────────────────────────────────────────┐               │
│             │        Unified Domain & Store (internal/db)        │               │
│             │  series | episodes | movies | indexers | queue     │               │
│             │  quality_profiles | root_folders | tags | history  │               │
│             └────────────────────────────────────────────────────┘               │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Unified SQLite Schema (`internal/db/migrations/001_init.sql`)
Replacearr stores all shared and media-specific state in a single WAL-mode SQLite database (`internal/db/sqlite.go`):
* **Shared Authentication & RBAC**: `roles`, `users`, `api_keys` (with `scope` supporting `all`, `sonarr`, `radarr`, `prowlarr` so ecosystem clients can use scoped or global API keys).
* **Shared Configuration Tables**:
  * `tags` (`id`, `label`): Single tag namespace across TV, Movies, Indexers, and Download Clients.
  * `root_folders` (`id`, `path`, `media_type`, `accessible`, `free_space`, `total_space`): Filtered by `media_type` (`tv` for Sonarr endpoints, `movie` for Radarr endpoints, or `all`).
  * `quality_profiles` (`id`, `name`, `upgrade_allowed`, `cutoff`, `items_json`, `min_format_score`, `cutoff_format_score`): Shared across media modules.
  * `indexers` (`id`, `name`, `implementation`, `protocol`, `privacy`, `enable`, `priority`, `base_url`, `definition_file`, `plugin_id`, `categories_json`, `fields_json`, `tags_json`): Single source of truth for all indexers.
  * `download_clients` (`id`, `name`, `implementation`, `protocol`, `enable`, `priority`, `host`, `port`, `category`, `fields_json`, `tags_json`): Single pool of download clients.
* **Discriminator-Backed Operational Tables**:
  * `queue_items` (`media_type` = `'tv'` | `'movie'`, `series_id`, `episode_id`, `movie_id`, `status`, `tracked_download_status`, `tracked_download_state`, `size`, `sizeleft`, `timeleft`, `protocol`, `download_client`, `download_id`, `indexer`, `output_path`).
  * `history_records` (`media_type` = `'tv'` | `'movie'`, `series_id`, `episode_id`, `movie_id`, `source_title`, `quality_json`, `date`, `event_type`, `data_json`).
* **Media Domain Tables**:
  * `series` and `episodes` (with `ON DELETE CASCADE` foreign key).
  * `movies`.

### 4.3 Direct In-Memory Indexer Execution vs. Compatibility Emulation
1. **Internal Search & RSS Sync**: When Replacearr executes a `SeriesSearch`, `SeasonSearch`, `EpisodeSearch`, `MoviesSearch`, or `RssSync` command, it queries the `indexers` table directly in-memory, executes the matching Cardigann YAML definition or Go indexer plugin, and feeds the resulting `domain.ReleaseResult` structs straight into the Decision Engine—zero HTTP loopback and zero XML serialization overhead.
2. **Ecosystem Compatibility**:
   * Querying `GET /sonarr/api/v3/indexer` or `GET /radarr/api/v3/indexer` reads directly from the unified `indexers` table, formatting each row as a `Torznab`/`Newznab` resource so third-party tools inspecting Sonarr/Radarr see all active indexers immediately.
   * Querying `GET /prowlarr/api/v1/applications` returns virtualized self-loopback application entries showing Sonarr and Radarr as pre-connected (`fullSync`) internal modules.
   * External *arr instances (if a user still runs an external Lidarr or Readarr container during migration) can still point to Replacearr's Torznab endpoint (`/{id}/api?t=caps|search|tvsearch|movie|music|book`).
