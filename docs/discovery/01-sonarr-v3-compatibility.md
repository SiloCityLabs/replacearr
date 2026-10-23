# 01 — Sonarr v3 API Compatibility Specification

## 1. Overview & Routing Prefix

External ecosystem tools (**Overseerr**, **Jellyseerr**, **Bazarr**, **nzb360**, **Homepage**, **Prowlarr**, **Recyclarr**, **Maintainerr**) interact with Sonarr using its v3 REST API.

In upstream Sonarr (`.tmp/Sonarr/src/Sonarr.Http/V3ApiControllerAttribute.cs`), every v3 controller is decorated with `[V3ApiController]`, which mounts routes at `/api/v3/[controller]`. Replacearr mounts the full Sonarr v3 compatibility surface at both:
* `/api/v3/*` (default root mount for drop-in Sonarr replacement)
* `/sonarr/api/v3/*` (virtual URL-base mount when sharing a single host/port with Radarr and Prowlarr clients)

---

## 2. Authentication Expectations

Upstream reference:
* `.tmp/Sonarr/src/Sonarr.Http/Authentication/ApiKeyAuthenticationHandler.cs`
* `.tmp/Sonarr/src/Sonarr.Http/Authentication/AuthenticationBuilderExtensions.cs`

Sonarr's `ApiKeyAuthenticationHandler.ParseApiKey()` resolves the API key in the following exact precedence order:
1. **Query Parameter**: `?apikey=<key>` (`Options.QueryName = "apikey"`, or `?access_token=<key>` for SignalR hubs).
2. **HTTP Header**: `X-Api-Key: <key>` (`Options.HeaderName = "X-Api-Key"`, case-insensitive in HTTP/1.1 & HTTP/2).
3. **Bearer Token**: `Authorization: Bearer <key>`.

If the key is missing or invalid, upstream returns `401 Unauthorized` (`Response.StatusCode = 401`). In Replacearr, any valid API key with `scope` in `("all", "sonarr")` and appropriate `tv:read` / `tv:write` permissions (or the configured `REPLACEARR_API_KEY`) satisfies authentication.

---

## 3. Required Endpoints & Upstream Controller Mapping

| Endpoint Path | Methods | Upstream Controller (`.tmp/Sonarr/src/Sonarr.Api.V3/`) | Primary Ecosystem Consumers |
| :--- | :--- | :--- | :--- |
| `/api/v3/system/status` | `GET` | `System/SystemController.cs` (`SystemResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360, Homepage, Prowlarr |
| `/api/v3/series` | `GET`, `POST` | `Series/SeriesController.cs` (`SeriesResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360, Homepage |
| `/api/v3/series/{id}` | `GET`, `PUT`, `DELETE` | `Series/SeriesController.cs` (`SeriesResource.cs`) | Overseerr, Jellyseerr, nzb360 |
| `/api/v3/series/lookup` | `GET` | `Series/SeriesLookupController.cs` | Overseerr, Jellyseerr, nzb360 |
| `/api/v3/episode` | `GET`, `PUT` | `Episodes/EpisodeController.cs` (`EpisodeResource.cs`) | Bazarr, nzb360, Overseerr |
| `/api/v3/episode/{id}` | `GET`, `PUT` | `Episodes/EpisodeController.cs` | nzb360, Bazarr |
| `/api/v3/episode/monitor` | `PUT` | `Episodes/EpisodeController.cs` (`EpisodesMonitoredResource`) | Overseerr, Jellyseerr, nzb360 |
| `/api/v3/episodefile` | `GET`, `DELETE` | `EpisodeFiles/EpisodeFileController.cs` (`EpisodeFileResource.cs`) | Bazarr (reads `path`, `mediaInfo`, `languages`), nzb360 |
| `/api/v3/episodefile/{id}` | `GET`, `PUT`, `DELETE` | `EpisodeFiles/EpisodeFileController.cs` | Bazarr, nzb360 |
| `/api/v3/calendar` | `GET` | `Calendar/CalendarController.cs` | Homepage, nzb360, Organizr |
| `/api/v3/queue` | `GET` | `Queue/QueueController.cs` (`QueueResource.cs`) | Homepage, nzb360, Overseerr |
| `/api/v3/queue/{id}` | `DELETE` | `Queue/QueueController.cs` | nzb360 |
| `/api/v3/queue/details` | `GET` | `Queue/QueueDetailsController.cs` | Sonarr UI, Overseerr |
| `/api/v3/command` | `GET`, `POST` | `Commands/CommandController.cs` (`CommandResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360 |
| `/api/v3/command/{id}` | `GET`, `DELETE` | `Commands/CommandController.cs` | Overseerr, nzb360 |
| `/api/v3/qualityprofile` | `GET`, `POST`, `PUT`, `DELETE` | `Profiles/Quality/QualityProfileController.cs` (`QualityProfileResource.cs`) | Overseerr, Jellyseerr, nzb360, Recyclarr |
| `/api/v3/languageprofile` | `GET`, `GET /{id}` | `Profiles/Languages/LanguageProfileController.cs` (`LanguageProfileResource.cs`) | Overseerr, Jellyseerr (legacy v3 requirement) |
| `/api/v3/rootfolder` | `GET`, `POST`, `DELETE` | `RootFolders/RootFolderController.cs` (`RootFolderResource.cs`) | Overseerr, Jellyseerr, nzb360 |
| `/api/v3/tag` | `GET`, `POST`, `PUT`, `DELETE` | `Tags/TagController.cs` (`TagResource.cs`) | Overseerr, Jellyseerr, Prowlarr, nzb360 |
| `/api/v3/downloadclient` | `GET`, `POST`, `PUT`, `DELETE` | `DownloadClient/DownloadClientController.cs` (`DownloadClientResource.cs`) | Prowlarr, nzb360 |
| `/api/v3/indexer` | `GET`, `POST`, `PUT`, `DELETE` | `Indexers/IndexerController.cs` (`IndexerResource.cs`) | Prowlarr (`SonarrV3Proxy`), nzb360 |
| `/api/v3/indexer/schema` | `GET` | `ProviderControllerBase.cs` (`GetTemplates()`) | Prowlarr (`Sonarr.BuildSonarrIndexer`) |
| `/api/v3/indexer/test` | `POST` | `ProviderControllerBase.cs` (`Test()`) | Prowlarr (`Sonarr.Test`) |
| `/api/v3/history` | `GET` | `History/HistoryController.cs` (`HistoryResource.cs`) | nzb360, Bazarr, Maintainerr |
| `/api/v3/history/series` | `GET` | `History/HistoryController.cs` | nzb360 |

---

## 4. Detailed Endpoint Contracts & JSON Payloads

### 4.1 `GET /api/v3/system/status`
* **Upstream Source**: `System/SystemController.cs` -> `System/SystemResource.cs`
* **Notes**: Overseerr/Jellyseerr and Prowlarr check `appName` and `version` (must start with `3.` or `4.`) to verify a valid Sonarr v3/v4 instance.
```json
{
  "appName": "Sonarr",
  "instanceName": "Replacearr (Sonarr Compat)",
  "version": "4.0.9.2244",
  "buildTime": "2026-10-23T00:00:00Z",
  "isDebug": false,
  "isProduction": true,
  "isAdmin": true,
  "isUserInteractive": false,
  "startupPath": "/app",
  "appData": "/config",
  "osName": "linux",
  "osVersion": "7.0.0",
  "isNetCore": true,
  "isLinux": true,
  "isOsx": false,
  "isWindows": false,
  "isDocker": true,
  "mode": "console",
  "branch": "main",
  "authentication": "apiKey",
  "sqliteVersion": "3.46.0",
  "migrationVersion": 200,
  "urlBase": "",
  "runtimeVersion": "8.0.0",
  "runtimeName": "replacearr-go",
  "startTime": "2026-10-23T12:00:00Z",
  "packageUpdateMechanism": "docker"
}
```

### 4.2 `/api/v3/series` and `/api/v3/series/lookup`
* **Upstream Source**: `Series/SeriesController.cs`, `Series/SeriesLookupController.cs`, `Series/SeriesResource.cs`
* **Query Parameters**:
  * `GET /api/v3/series?tvdbId={tvdbId}&includeSeasonImages={bool}`
  * `GET /api/v3/series/lookup?term={term}` (supports `tvdb:12345` or free-text title query)
  * `DELETE /api/v3/series/{id}?deleteFiles={bool}&addImportListExclusion={bool}`
* **POST Request / GET Response Shape (`SeriesResource`)**:
  * When Overseerr/Jellyseerr adds a series via `POST /api/v3/series`, it sends `title`, `tvdbId`, `qualityProfileId`, `languageProfileId`, `rootFolderPath`, `seasonFolder`, `monitored`, `seriesType` (`standard`, `daily`, `anime`), `seasons`, `tags`, and `addOptions`:
```json
{
  "id": 1,
  "title": "Severance",
  "sortTitle": "severance",
  "status": "continuing",
  "ended": false,
  "overview": "Mark leads a team of office workers whose memories have been surgically divided...",
  "network": "Apple TV+",
  "airTime": "21:00",
  "images": [
    {
      "coverType": "poster",
      "url": "/MediaCover/1/poster.jpg",
      "remoteUrl": "https://artworks.thetvdb.com/banners/v4/series/371980/posters/61f84f8e.jpg"
    }
  ],
  "seasons": [
    {
      "seasonNumber": 1,
      "monitored": true,
      "statistics": {
        "episodeFileCount": 9,
        "episodeCount": 9,
        "totalEpisodeCount": 9,
        "sizeOnDisk": 19327352832,
        "percentOfEpisodes": 100.0
      }
    }
  ],
  "year": 2022,
  "path": "/data/tv/Severance",
  "qualityProfileId": 1,
  "languageProfileId": 1,
  "seasonFolder": true,
  "monitored": true,
  "monitorNewItems": "all",
  "useSceneNumbering": false,
  "runtime": 50,
  "tvdbId": 371980,
  "tvRageId": 0,
  "tvMazeId": 44933,
  "tmdbId": 95396,
  "imdbId": "tt11280740",
  "titleSlug": "severance",
  "rootFolderPath": "/data/tv",
  "seriesType": "standard",
  "cleanTitle": "severance",
  "genres": ["Drama", "Mystery", "Science Fiction"],
  "tags": [1],
  "added": "2026-10-23T12:00:00Z",
  "addOptions": {
    "monitor": "all",
    "searchForMissingEpisodes": true,
    "searchForCutoffUnmetEpisodes": false
  },
  "statistics": {
    "seasonCount": 1,
    "episodeFileCount": 9,
    "episodeCount": 9,
    "totalEpisodeCount": 9,
    "sizeOnDisk": 19327352832,
    "percentOfEpisodes": 100.0
  }
}
```
* **Important Implementation Behavior**: In upstream `AddSeriesService.cs`, if `addOptions.searchForMissingEpisodes == true` on `POST /api/v3/series`, Sonarr automatically triggers a `SeriesSearchCommand` or `MissingEpisodeSearchCommand`.

### 4.3 `/api/v3/episode` and `/api/v3/episodefile`
* **Upstream Source**: `Episodes/EpisodeController.cs`, `Episodes/EpisodeResource.cs`, `EpisodeFiles/EpisodeFileController.cs`, `EpisodeFiles/EpisodeFileResource.cs`
* **Query Parameters**:
  * `GET /api/v3/episode?seriesId={id}&seasonNumber={num}&episodeIds={ids}&episodeFileId={fileId}&includeSeries={bool}&includeEpisodeFile={bool}&includeImages={bool}`
  * `PUT /api/v3/episode/monitor` with body `{"episodeIds": [1, 2, 3], "monitored": true}`
  * `GET /api/v3/episodefile?seriesId={id}&episodeFileIds={ids}`
* **`EpisodeResource` JSON Shape**:
```json
{
  "id": 101,
  "seriesId": 1,
  "tvdbId": 8912345,
  "episodeFileId": 50,
  "seasonNumber": 1,
  "episodeNumber": 1,
  "title": "Good News About Hell",
  "airDate": "2022-02-18",
  "airDateUtc": "2022-02-18T02:00:00Z",
  "runtime": 57,
  "overview": "Mark is promoted to lead a team...",
  "hasFile": true,
  "monitored": true,
  "absoluteEpisodeNumber": 1,
  "unverifiedSceneNumbering": false
}
```
* **`EpisodeFileResource` JSON Shape** (critical for **Bazarr** subtitle matching):
```json
{
  "id": 50,
  "seriesId": 1,
  "seasonNumber": 1,
  "relativePath": "Season 01/Severance - S01E01 - Good News About Hell WEBDL-1080p.mkv",
  "path": "/data/tv/Severance/Season 01/Severance - S01E01 - Good News About Hell WEBDL-1080p.mkv",
  "size": 2147483648,
  "dateAdded": "2026-10-23T12:15:00Z",
  "sceneName": "Severance.S01E01.Good.News.About.Hell.1080p.ATVP.WEB-DL.DDP5.1.Atmos.H.264-FLUX",
  "releaseGroup": "FLUX",
  "languages": [{ "id": 1, "name": "English" }],
  "quality": {
    "quality": { "id": 3, "name": "WEBDL-1080p", "source": "web", "resolution": 1080 },
    "revision": { "version": 1, "real": 0, "isRepack": false }
  },
  "customFormats": [],
  "customFormatScore": 0,
  "qualityCutoffNotMet": false
}
```

### 4.4 `GET /api/v3/calendar`
* **Upstream Source**: `Calendar/CalendarController.cs`
* **Query Parameters**: `?start=2026-10-23T00:00:00Z&end=2026-10-30T00:00:00Z&unmonitored=false&includeSeries=true&includeEpisodeFile=false&includeEpisodeImages=false&tags=`
* **Response**: Array of `EpisodeResource` objects (`List<EpisodeResource>`) ordered by `airDateUtc` ascending, optionally embedding `"series": { ... }` when `includeSeries=true` (used by **Homepage** and **nzb360**).

### 4.5 `/api/v3/queue` and `/api/v3/history` (Paginated Envelopes)
* **Upstream Source**: `Queue/QueueController.cs` (`QueueResource.cs`), `History/HistoryController.cs` (`HistoryResource.cs`), `Sonarr.Http/PagingResource.cs`
* **Important Note**: Both `/api/v3/queue` and `/api/v3/history` return a **`PagingResource<T>`** wrapper object (not a raw array!):
  * Query Params: `?page=1&pageSize=20&sortKey=timeleft&sortDirection=ascending&includeUnknownSeriesItems=false&includeSeries=true&includeEpisode=true`
```json
{
  "page": 1,
  "pageSize": 20,
  "sortKey": "timeleft",
  "sortDirection": "ascending",
  "totalRecords": 1,
  "records": [
    {
      "id": 1,
      "seriesId": 1,
      "episodeId": 101,
      "seasonNumber": 1,
      "title": "Severance.S01E01.1080p.WEB-DL-FLUX",
      "size": 2147483648,
      "sizeleft": 536870912,
      "timeleft": "00:04:12",
      "estimatedCompletionTime": "2026-10-23T16:45:00Z",
      "added": "2026-10-23T16:30:00Z",
      "status": "downloading",
      "trackedDownloadStatus": "ok",
      "trackedDownloadState": "downloading",
      "statusMessages": [],
      "downloadId": "A1B2C3D4E5F6",
      "protocol": "torrent",
      "downloadClient": "qBittorrent",
      "indexer": "YTS",
      "outputPath": "/data/downloads/Severance.S01E01",
      "episodeHasFile": false
    }
  ]
}
```
* **Non-Paginated History Sub-Routes**:
  * `GET /api/v3/history/since?date=2026-10-20T00:00:00Z&eventType=1&includeSeries=false&includeEpisode=false` -> Returns `List<HistoryResource>` (raw JSON array).
  * `GET /api/v3/history/series?seriesId=1&seasonNumber=1&eventType=1` -> Returns `List<HistoryResource>` (raw JSON array).

### 4.6 `/api/v3/command`
* **Upstream Source**: `Commands/CommandController.cs`, `Commands/CommandResource.cs`, `NzbDrone.Core/IndexerSearch/*SearchCommand.cs`, `NzbDrone.Core/Tv/Commands/RefreshSeriesCommand.cs`, `NzbDrone.Core/MediaFiles/Commands/RescanSeriesCommand.cs`, `NzbDrone.Core/Indexers/RssSyncCommand.cs`
* **`POST /api/v3/command` Supported Command Payloads**:
  * **`SeriesSearch`** (`IndexerSearch/SeriesSearchCommand.cs`):
    ```json
    { "name": "SeriesSearch", "seriesId": 1 }
    ```
  * **`SeasonSearch`** (`IndexerSearch/SeasonSearchCommand.cs`):
    ```json
    { "name": "SeasonSearch", "seriesId": 1, "seasonNumber": 1 }
    ```
  * **`EpisodeSearch`** (`IndexerSearch/EpisodeSearchCommand.cs`):
    ```json
    { "name": "EpisodeSearch", "episodeIds": [101, 102] }
    ```
  * **`RefreshSeries`** (`Tv/Commands/RefreshSeriesCommand.cs` — supports both `seriesId` single int and `seriesIds` int array):
    ```json
    { "name": "RefreshSeries", "seriesId": 1, "isNewSeries": false }
    ```
  * **`RescanSeries`** (`MediaFiles/Commands/RescanSeriesCommand.cs`):
    ```json
    { "name": "RescanSeries", "seriesId": 1 }
    ```
  * **`RssSync`** (`Indexers/RssSyncCommand.cs`):
    ```json
    { "name": "RssSync" }
    ```
* **Response (`201 Created`, `CommandResource`)**:
```json
{
  "id": 42,
  "name": "SeriesSearch",
  "commandName": "Series Search",
  "message": "Completed",
  "body": {
    "seriesId": 1,
    "sendUpdatesToClient": true,
    "updateScheduledTask": false,
    "completionMessage": "Completed",
    "requiresDiskAccess": false,
    "isExclusive": false,
    "isLongRunning": false,
    "name": "SeriesSearch",
    "trigger": "manual"
  },
  "priority": "normal",
  "status": "queued",
  "result": "unknown",
  "queued": "2026-10-23T16:36:00Z",
  "started": "2026-10-23T16:36:00Z",
  "ended": "2026-10-23T16:36:01Z",
  "trigger": "manual",
  "stateChangeTime": "2026-10-23T16:36:00Z",
  "sendUpdatesToClient": true,
  "updateScheduledTask": false
}
```

### 4.7 `/api/v3/qualityprofile`, `/api/v3/languageprofile`, `/api/v3/rootfolder`, `/api/v3/tag`
* **`GET /api/v3/qualityprofile`** (`Profiles/Quality/QualityProfileController.cs`):
  Returns `List<QualityProfileResource>` with `id`, `name`, `upgradeAllowed`, `cutoff`, `minFormatScore`, `cutoffFormatScore`, `minUpgradeFormatScore`, `formatItems`, and `items` (where each item has `quality: { id, name, source, resolution }`, `items: []`, `allowed: true`).
* **`GET /api/v3/languageprofile`** (`Profiles/Languages/LanguageProfileController.cs`):
  Marked `[Obsolete("Deprecated")]` in Sonarr v4, but **still required by Overseerr and Jellyseerr** when configuring a Sonarr server! Upstream hardcodes a single profile response:
  ```json
  [
    {
      "id": 1,
      "name": "Deprecated",
      "upgradeAllowed": true,
      "cutoff": { "id": 1, "name": "English" },
      "languages": [
        { "id": 1, "language": { "id": 1, "name": "English" }, "allowed": true }
      ]
    }
  ]
  ```
* **`GET /api/v3/rootfolder`** (`RootFolders/RootFolderController.cs`):
  Returns `List<RootFolderResource>` filtered to TV/shared root folders (`id`, `path`, `accessible`, `freeSpace`, `totalSpace`, `unmappedFolders: []`).
* **`GET /api/v3/tag`** (`Tags/TagController.cs`):
  Returns `List<TagResource>` (`[{"id": 1, "label": "4k"}]`).

### 4.8 `/api/v3/downloadclient` and `/api/v3/indexer`
* **Upstream Source**: `DownloadClient/DownloadClientController.cs`, `Indexers/IndexerController.cs`, `ProviderControllerBase.cs`
* Both inherit from `ProviderControllerBase` and use the `ProviderResource` schema (`id`, `name`, `implementation`, `implementationName`, `configContract`, `infoLink`, `tags`, `fields: [{ "name": "...", "label": "...", "value": "...", "type": "..." }]`).
* `IndexerResource` adds `enableRss`, `enableAutomaticSearch`, `enableInteractiveSearch`, `supportsRss`, `supportsSearch`, `protocol` (`torrent` | `usenet`), `priority` (`1..50`), `seasonSearchMaximumSingleEpisodeAge`, and `downloadClientId`.
* Supporting `GET /api/v3/indexer/schema` and `POST /api/v3/indexer/test` ensures external Prowlarr instances (if any) can test and sync indexers without error.
