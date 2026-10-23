# 02 — Radarr v3 API Compatibility Specification

## 1. Overview & Routing Prefix

External ecosystem tools (**Overseerr**, **Jellyseerr**, **Bazarr**, **nzb360**, **Homepage**, **Prowlarr**, **Recyclarr**, **Maintainerr**) communicate with Radarr using its v3 REST API.

In upstream Radarr (`.tmp/Radarr/src/Radarr.Http/V3ApiControllerAttribute.cs`), every v3 controller is decorated with `[V3ApiController]`, mounting routes at `/api/v3/[controller]`. Replacearr exposes the full Radarr v3 compatibility surface at:
* `/radarr/api/v3/*` (dedicated URL-base mount for Radarr clients)
* `/api/v3/movie*` (also mounted on the root `/api/v3` router since `/movie`, `/movie/lookup`, and `/moviefile` do not collide with Sonarr's `/series`, `/episode`, and `/episodefile` paths!)

Authentication is identical to Sonarr v3 (`.tmp/Radarr/src/Radarr.Http/Authentication/ApiKeyAuthenticationHandler.cs`):
1. Query parameter `?apikey=<key>`
2. Header `X-Api-Key: <key>`
3. Header `Authorization: Bearer <key>`

---

## 2. Structural Comparison: Sonarr v3 vs. Radarr v3

While Sonarr v3 and Radarr v3 share `NzbDrone.Core` and `ProviderControllerBase`, their media domain models, lookup routes,file endpoints, and command payloads differ in critical ways that Replacearr's compatibility layer handles explicitly:

| Feature / Dimension | Sonarr v3 (`.tmp/Sonarr/src/Sonarr.Api.V3`) | Radarr v3 (`.tmp/Radarr/src/Radarr.Api.V3`) |
| :--- | :--- | :--- |
| **Primary Media Entity** | `SeriesResource` (`/api/v3/series`), hierarchical (`Series` -> `Season` -> `Episode` -> `EpisodeFile`) | `MovieResource` (`/api/v3/movie`), flat 1-to-1 (`Movie` -> `MovieFile`) |
| **Primary External ID** | `tvdbId` (integer, required on `POST /series` and `GET /series?tvdbId=`) | `tmdbId` (integer, required on `POST /movie` and `GET /movie?tmdbId=`) |
| **Lookup Endpoints** | `GET /api/v3/series/lookup?term=...` (single endpoint; supports `tvdb:12345` in `term`) | `GET /api/v3/movie/lookup?term=...` **PLUS** dedicated sub-routes:<br>• `GET /api/v3/movie/lookup/tmdb?tmdbId={id}`<br>• `GET /api/v3/movie/lookup/imdb?imdbId={tt...}` |
| **Availability & Status** | `status`: `"continuing"`, `"ended"`, `"upcoming"`, `"deleted"`<br>`seriesType`: `"standard"`, `"daily"`, `"anime"` | `status`: `"tba"`, `"announced"`, `"inCinemas"`, `"released"`, `"deleted"`<br>`minimumAvailability`: `"announced"`, `"inCinemas"`, `"released"`<br>Release dates: `inCinemas`, `physicalRelease`, `digitalRelease` |
| **Add Options (`addOptions`)** | `{ "monitor": "all", "searchForMissingEpisodes": true, "searchForCutoffUnmetEpisodes": false }` | `{ "monitor": "movieOnly", "searchForMovie": true, "addMethod": "manual" }` |
| **Media File Query Param** | `GET /api/v3/episodefile?seriesId=1&episodeFileIds=1,2` (`seriesId` is a single `int?`) | `GET /api/v3/moviefile?movieId=1&movieFileIds=1,2` (`movieId` binds to `List<int> movieIds`) |
| **Media File Properties** | `EpisodeFileResource` has `seriesId`, `seasonNumber`, `releaseType` | `MovieFileResource` has `movieId`, `edition` (e.g., `"Director's Cut"`), `originalFilePath` |
| **Language Profiles** | Exposes deprecated `/api/v3/languageprofile`; `SeriesResource` includes `languageProfileId: 1` | **No `/languageprofile` controller** (removed in Radarr v3; language is configured directly on `QualityProfile` or `OriginalLanguage`) |
| **Calendar Response** | `GET /api/v3/calendar` returns `List<EpisodeResource>` ordered by `airDateUtc` | `GET /api/v3/calendar` returns `List<MovieResource>` ordered by `inCinemas`, then `digitalRelease`, then `physicalRelease` |
| **Queue Item Shape** | `QueueResource` has `seriesId`, `episodeId`, `seasonNumber`, `series`, `episode` | `QueueResource` has `movieId`, `movie` |
| **Search & Refresh Commands** | `SeriesSearch` (`seriesId`), `SeasonSearch` (`seriesId`, `seasonNumber`), `EpisodeSearch` (`episodeIds`), `RefreshSeries` (`seriesIds`/`seriesId`), `RescanSeries` (`seriesId`) | `MoviesSearch` (`movieIds`: `List<int>`), `RefreshMovie` (`movieIds`: `List<int>`), `RescanMovie` (`movieId`: `int?`), `RssSync` |

---

## 3. Required Endpoints & Upstream Controller Mapping

| Endpoint Path | Methods | Upstream Controller (`.tmp/Radarr/src/Radarr.Api.V3/`) | Primary Ecosystem Consumers |
| :--- | :--- | :--- | :--- |
| `/api/v3/system/status` | `GET` | `System/SystemController.cs` (`SystemResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360, Homepage, Prowlarr |
| `/api/v3/movie` | `GET`, `POST` | `Movies/MovieController.cs` (`MovieResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360, Homepage |
| `/api/v3/movie/{id}` | `GET`, `PUT`, `DELETE` | `Movies/MovieController.cs` (`MovieResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360 |
| `/api/v3/movie/editor` | `PUT`, `DELETE` | `Movies/MovieEditorController.cs` | Radarr UI, nzb360, Maintainerr |
| `/api/v3/movie/lookup` | `GET` | `Movies/MovieLookupController.cs` | Overseerr, Jellyseerr, nzb360 |
| `/api/v3/movie/lookup/tmdb` | `GET` | `Movies/MovieLookupController.cs` (`SearchByTmdbId`) | Overseerr, Jellyseerr |
| `/api/v3/movie/lookup/imdb` | `GET` | `Movies/MovieLookupController.cs` (`SearchByImdbId`) | nzb360, external scripts |
| `/api/v3/moviefile` | `GET`, `DELETE` | `MovieFiles/MovieFileController.cs` (`MovieFileResource.cs`) | Bazarr, nzb360 |
| `/api/v3/moviefile/{id}` | `GET`, `PUT`, `DELETE` | `MovieFiles/MovieFileController.cs` | Bazarr, nzb360 |
| `/api/v3/calendar` | `GET` | `Calendar/CalendarController.cs` | Homepage, nzb360 |
| `/api/v3/queue` | `GET` | `Queue/QueueController.cs` (`QueueResource.cs`) | Homepage, nzb360, Overseerr |
| `/api/v3/queue/{id}` | `DELETE` | `Queue/QueueController.cs` | nzb360 |
| `/api/v3/queue/details` | `GET` | `Queue/QueueDetailsController.cs` | Overseerr, Radarr UI |
| `/api/v3/command` | `GET`, `POST` | `Commands/CommandController.cs` (`CommandResource.cs`) | Overseerr, Jellyseerr, Bazarr, nzb360 |
| `/api/v3/command/{id}` | `GET`, `DELETE` | `Commands/CommandController.cs` | Overseerr, nzb360 |
| `/api/v3/qualityprofile` | `GET`, `POST`, `PUT`, `DELETE` | `Profiles/Quality/QualityProfileController.cs` (`QualityProfileResource.cs`) | Overseerr, Jellyseerr, nzb360, Recyclarr |
| `/api/v3/rootfolder` | `GET`, `POST`, `DELETE` | `RootFolders/RootFolderController.cs` (`RootFolderResource.cs`) | Overseerr, Jellyseerr, nzb360 |
| `/api/v3/tag` | `GET`, `POST`, `PUT`, `DELETE` | `Tags/TagController.cs` (`TagResource.cs`) | Overseerr, Jellyseerr, Prowlarr, nzb360 |
| `/api/v3/downloadclient` | `GET`, `POST`, `PUT`, `DELETE` | `DownloadClient/DownloadClientController.cs` | Prowlarr, nzb360 |
| `/api/v3/indexer` | `GET`, `POST`, `PUT`, `DELETE` | `Indexers/IndexerController.cs` | Prowlarr (`RadarrV3Proxy`), nzb360 |
| `/api/v3/indexer/schema` | `GET` | `ProviderControllerBase.cs` (`GetTemplates()`) | Prowlarr (`Radarr.BuildRadarrIndexer`) |
| `/api/v3/indexer/test` | `POST` | `ProviderControllerBase.cs` (`Test()`) | Prowlarr (`Radarr.Test`) |
| `/api/v3/history` | `GET` | `History/HistoryController.cs` (`HistoryResource.cs`) | nzb360, Bazarr, Maintainerr |
| `/api/v3/history/movie` | `GET` | `History/HistoryController.cs` (`GetMovieHistory`) | nzb360, Bazarr |

---

## 4. Detailed Endpoint Contracts & JSON Payloads

### 4.1 `GET /radarr/api/v3/system/status`
* **Upstream Source**: `.tmp/Radarr/src/Radarr.Api.V3/System/SystemController.cs`
* **Response**: Same structure as Sonarr's `SystemResource`, with `"appName": "Radarr"`, `"instanceName": "Replacearr (Radarr Compat)"`, and `"version": "5.14.0.9383"`.

### 4.2 `/api/v3/movie` and `/api/v3/movie/lookup`
* **Upstream Source**: `Movies/MovieController.cs`, `Movies/MovieLookupController.cs`, `Movies/MovieResource.cs`
* **Query Parameters**:
  * `GET /api/v3/movie?tmdbId={tmdbId}&excludeLocalCovers={bool}&languageId={int}`
  * `GET /api/v3/movie/lookup?term={term}` (supports `tmdb:12345`, `imdb:tt1234567`, or free-text search)
  * `GET /api/v3/movie/lookup/tmdb?tmdbId={tmdbId}` (returns a single `MovieResource` object, **not** an array! Overseerr calls this directly before adding a movie)
  * `GET /api/v3/movie/lookup/imdb?imdbId={imdbId}` (returns a single `MovieResource` object)
  * `DELETE /api/v3/movie/{id}?deleteFiles={bool}&addImportExclusion={bool}` (note: parameter name is `addImportExclusion` in Radarr vs. `addImportListExclusion` in Sonarr)
* **POST Request / GET Response Shape (`MovieResource`)**:
```json
{
  "id": 1,
  "title": "Dune: Part Two",
  "originalTitle": "Dune: Part Two",
  "originalLanguage": { "id": 1, "name": "English" },
  "alternateTitles": [],
  "sortTitle": "dune part two",
  "sizeOnDisk": 15461882265,
  "status": "released",
  "overview": "Follow the mythic journey of Paul Atreides as he unites with Chani and the Fremen...",
  "inCinemas": "2024-02-27T00:00:00Z",
  "physicalRelease": "2024-05-14T00:00:00Z",
  "digitalRelease": "2024-04-16T00:00:00Z",
  "releaseDate": "2024-04-16T00:00:00Z",
  "images": [
    {
      "coverType": "poster",
      "url": "/MediaCover/Movies/1/poster.jpg",
      "remoteUrl": "https://image.tmdb.org/t/p/original/1pdfLvkbY9ohJlCjQH2CZjjYVvJ.jpg"
    }
  ],
  "website": "https://www.dunemovie.com",
  "year": 2024,
  "youTubeTrailerId": "Way9Dexny3w",
  "studio": "Legendary Pictures",
  "path": "/data/movies/Dune Part Two (2024)",
  "qualityProfileId": 1,
  "hasFile": true,
  "movieFileId": 10,
  "monitored": true,
  "minimumAvailability": "released",
  "isAvailable": true,
  "folderName": "Dune Part Two (2024)",
  "runtime": 167,
  "cleanTitle": "duneparttwo",
  "imdbId": "tt15239678",
  "tmdbId": 693134,
  "titleSlug": "693134",
  "rootFolderPath": "/data/movies",
  "certification": "PG-13",
  "genres": ["Science Fiction", "Adventure"],
  "tags": [1],
  "added": "2026-10-23T12:00:00Z",
  "addOptions": {
    "monitor": "movieOnly",
    "searchForMovie": true,
    "addMethod": "manual"
  },
  "statistics": {
    "movieFileCount": 1,
    "sizeOnDisk": 15461882265,
    "releaseGroups": ["FLUX"]
  }
}
```

### 4.3 `/api/v3/moviefile`
* **Upstream Source**: `MovieFiles/MovieFileController.cs`, `MovieFiles/MovieFileResource.cs`
* **Query Parameters**:
  * `GET /api/v3/moviefile?movieId=1` or `GET /api/v3/moviefile?movieFileIds=10,11`
* **`MovieFileResource` JSON Shape**:
```json
{
  "id": 10,
  "movieId": 1,
  "relativePath": "Dune.Part.Two.2024.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-FLUX.mkv",
  "path": "/data/movies/Dune Part Two (2024)/Dune.Part.Two.2024.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-FLUX.mkv",
  "size": 15461882265,
  "dateAdded": "2026-10-23T13:00:00Z",
  "sceneName": "Dune.Part.Two.2024.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-FLUX",
  "releaseGroup": "FLUX",
  "edition": "",
  "languages": [{ "id": 1, "name": "English" }],
  "quality": {
    "quality": { "id": 18, "name": "WEBDL-2160p", "source": "web", "resolution": 2160 },
    "revision": { "version": 1, "real": 0, "isRepack": false }
  },
  "customFormats": [],
  "customFormatScore": 0,
  "indexerFlags": 0,
  "qualityCutoffNotMet": false
}
```

### 4.4 `GET /api/v3/calendar`
* **Upstream Source**: `Calendar/CalendarController.cs`
* **Query Parameters**: `?start=2026-10-23T00:00:00Z&end=2026-10-30T00:00:00Z&unmonitored=false&tags=`
* **Response**: Returns `List<MovieResource>` whose `inCinemas`, `digitalRelease`, or `physicalRelease` falls between `start` and `end`, ordered by `inCinemas` -> `digitalRelease` -> `physicalRelease`.

### 4.5 `/api/v3/queue` and `/api/v3/history`
* **Upstream Source**: `Queue/QueueController.cs` (`QueueResource.cs`), `History/HistoryController.cs` (`HistoryResource.cs`)
* **`GET /api/v3/queue`**:
  * Query Params: `?page=1&pageSize=20&sortKey=timeleft&sortDirection=ascending&includeUnknownMovieItems=false&includeMovie=true`
  * Returns `PagingResource<QueueResource>` filtered to `media_type = 'movie'` in Replacearr's `queue_items` table, with `movieId` and optional `movie` object.
* **`GET /api/v3/history`**:
  * Query Params: `?page=1&pageSize=20&sortKey=date&sortDirection=descending&includeMovie=true&eventType=1&movieIds=1`
  * Returns `PagingResource<HistoryResource>` filtered to `media_type = 'movie'`.
* **`GET /api/v3/history/movie?movieId=1&eventType=1&includeMovie=false`**:
  * Returns non-paginated `List<HistoryResource>` for a specific movie.

### 4.6 `/api/v3/command` (Radarr v3 Commands)
* **Upstream Source**: `Commands/CommandController.cs`, `NzbDrone.Core/IndexerSearch/MoviesSearchCommand.cs`, `NzbDrone.Core/Movies/Commands/RefreshMovieCommand.cs`, `NzbDrone.Core/MediaFiles/Commands/RescanMovieCommand.cs`, `NzbDrone.Core/Indexers/RssSyncCommand.cs`
* **Supported Command Payloads**:
  * **`MoviesSearch`** (`IndexerSearch/MoviesSearchCommand.cs` — note plural `MoviesSearch` and `movieIds` array!):
    ```json
    { "name": "MoviesSearch", "movieIds": [1, 2] }
    ```
  * **`RefreshMovie`** (`Movies/Commands/RefreshMovieCommand.cs` — supports `movieIds: [1]` or `movieId: 1`):
    ```json
    { "name": "RefreshMovie", "movieIds": [1], "isNewMovie": false }
    ```
  * **`RescanMovie`** (`MediaFiles/Commands/RescanMovieCommand.cs`):
    ```json
    { "name": "RescanMovie", "movieId": 1 }
    ```
  * **`RssSync`** (`Indexers/RssSyncCommand.cs`):
    ```json
    { "name": "RssSync" }
    ```
