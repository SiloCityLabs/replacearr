# 03 — Prowlarr v1, Torznab/Newznab & Cardigann Compatibility Specification

## 1. Overview & Routing Prefix

Upstream **Prowlarr** uses API v1 (`[V1ApiController]` in `.tmp/Prowlarr/src/Prowlarr.Http/V1ApiControllerAttribute.cs`) mounted at `/api/v1/[controller]`, plus top-level Torznab/Newznab feed routes (`/{id:int}/api` and `/api/v1/indexer/{id:int}/newznab`).

Replacearr exposes Prowlarr v1 compatibility at:
* `/prowlarr/api/v1/*` (dedicated Prowlarr v1 mount)
* `/api/v1/*` (shared v1 router where Prowlarr endpoints coexist with Replacearr's native v1 endpoints)
* `/{id}/api` and `/prowlarr/{id}/api` and `/api/v1/indexer/{id}/newznab` (Torznab/Newznab XML/JSON protocol endpoints)
* `/{id}/download` and `/api/v1/indexer/{id}/download` (Release proxy/redirect download endpoints)

Authentication accepts `?apikey=<key>`, `X-Api-Key: <key>`, or `Authorization: Bearer <key>`.

---

## 2. Required Prowlarr v1 REST Endpoints & Upstream Mapping

| Endpoint Path | Methods | Upstream Controller (`.tmp/Prowlarr/src/Prowlarr.Api.V1/`) | Purpose & Ecosystem Consumers |
| :--- | :--- | :--- | :--- |
| `/api/v1/system/status` | `GET` | `System/SystemController.cs` (`SystemResource.cs`) | nzb360, Homepage, Prowlarr UI/health checks (`"appName": "Prowlarr"`, `"version": "1.24.2.4771"`) |
| `/api/v1/indexer` | `GET`, `POST`, `PUT`, `DELETE` | `Indexers/IndexerController.cs` (`IndexerResource.cs`) | Manage Torznab, Newznab, Cardigann, and Plugin indexers |
| `/api/v1/indexer/schema` | `GET` | `ProviderControllerBase.cs` (`GetTemplates()`) | List all available native + Cardigann indexer templates (`~580+` definitions) |
| `/api/v1/indexer/test` | `POST` | `ProviderControllerBase.cs` (`Test()`) | Validate indexer connectivity and search execution |
| `/api/v1/indexerstatus` | `GET` | `Indexers/IndexerStatusController.cs` (`IndexerStatusResource.cs`) | Health/backoff state for rate-limited or failing indexers (`disabledTill`, `mostRecentFailure`) |
| `/api/v1/search` | `GET`, `POST` | `Search/SearchController.cs` (`ReleaseResource.cs`, `SearchResource.cs`) | Interactive cross-indexer search (`GET`) and release grab (`POST`) |
| `/api/v1/search/bulk` | `POST` | `Search/SearchController.cs` (`GrabReleases`) | Bulk grab multiple releases across indexers |
| `/api/v1/applications` | `GET`, `POST`, `PUT`, `DELETE` | `Applications/ApplicationController.cs` (`ApplicationResource.cs`) | Sync configuration for target *arr apps (returns virtualized internal Sonarr/Radarr sync status in Replacearr) |
| `/api/v1/downloadclient` | `GET`, `POST`, `PUT`, `DELETE` | `DownloadClient/DownloadClientController.cs` | Manage download clients for direct Prowlarr grabs |
| `/api/v1/tag` | `GET`, `POST`, `PUT`, `DELETE` | `Tags/TagController.cs` (`TagResource.cs`) | Tag management shared with indexers and applications |
| `/api/v1/indexerstats` | `GET` | `Indexers/IndexerStatsController.cs` | Homepage widget & Prowlarr statistics |

---

## 3. Prowlarr v1 JSON Payload Shapes

### 3.1 `/api/v1/indexer` (`IndexerResource.cs`)
Upstream reference: `.tmp/Prowlarr/src/Prowlarr.Api.V1/Indexers/IndexerResource.cs`
```json
{
  "id": 1,
  "name": "YTS",
  "implementationName": "Cardigann",
  "implementation": "Cardigann",
  "configContract": "CardigannSettings",
  "infoLink": "https://wiki.servarr.com/prowlarr/supported-indexers#yts",
  "definitionName": "yts",
  "description": "YTS is a Public torrent site specialising in HD movies of small size",
  "language": "en-US",
  "encoding": "Unicode (UTF-8)",
  "enable": true,
  "redirect": false,
  "supportsRss": true,
  "supportsSearch": true,
  "supportsRedirect": false,
  "supportsPagination": false,
  "appProfileId": 1,
  "protocol": "torrent",
  "privacy": "public",
  "priority": 25,
  "downloadClientId": 0,
  "added": "2026-10-23T12:00:00Z",
  "sortName": "yts",
  "indexerUrls": ["https://yts.gg/", "https://yts.ninjaproxy1.com/"],
  "legacyUrls": ["https://yts.mx/", "https://yts.lt/"],
  "tags": [],
  "fields": [
    { "name": "definitionFile", "value": "yts" },
    { "name": "baseUrl", "label": "Base Url", "value": "https://yts.gg/", "type": "select" },
    { "name": "apiurl", "label": "API URL", "value": "movies-api.accel.li", "type": "textbox" }
  ],
  "capabilities": {
    "limitsMax": 100,
    "limitsDefault": 100,
    "supportsRawSearch": false,
    "searchParams": ["q"],
    "tvSearchParams": [],
    "movieSearchParams": ["q", "imdbId"],
    "musicSearchParams": [],
    "bookSearchParams": [],
    "categories": [
      {
        "id": 2000,
        "name": "Movies",
        "subCategories": [
          { "id": 2040, "name": "Movies/HD" },
          { "id": 2045, "name": "Movies/UHD" },
          { "id": 2060, "name": "Movies/3D" }
        ]
      }
    ]
  }
}
```

### 3.2 `/api/v1/indexerstatus` (`IndexerStatusResource.cs`)
Upstream reference: `.tmp/Prowlarr/src/Prowlarr.Api.V1/Indexers/IndexerStatusResource.cs`
```json
[
  {
    "id": 1,
    "indexerId": 3,
    "disabledTill": "2026-10-23T17:00:00Z",
    "mostRecentFailure": "2026-10-23T16:45:00Z",
    "initialFailure": "2026-10-23T16:30:00Z"
  }
]
```

### 3.3 `/api/v1/search` (`SearchController.cs` & `ReleaseResource.cs`)
* **GET Query Parameters (`SearchResource.cs`)**:
  `?query=Severance&type=search|tvsearch|movie|music|book&indexerIds=1,2&categories=5000,5040&limit=100&offset=0`
* **Note on Embedded Query Tokens (`NewznabRequest.cs`)**:
  When `type=tvsearch` or `type=movie`, Prowlarr parses inline brace tokens inside `query` via `NewznabRequest.QueryToParams()`:
  * TV tokens: `{imdbid:tt11280740}`, `{tvdbid:371980}`, `{tmdbid:95396}`, `{tvmazeid:44933}`, `{season:1}`, `{episode:1}`, `{year:2022}`
  * Movie tokens: `{imdbid:tt15239678}`, `{tmdbid:693134}`, `{traktid:...}`, `{year:2024}`
* **GET Response / POST Grab Request (`ReleaseResource`)**:
```json
[
  {
    "guid": "https://yts.gg/torrent/download/ABCDEF123456",
    "age": 12,
    "ageHours": 288.5,
    "ageMinutes": 17310.0,
    "size": 2147483648,
    "files": 1,
    "grabs": 450,
    "indexerId": 1,
    "indexer": "YTS",
    "title": "Dune Part Two (2024) 1080p WEBRip 5.1 10Bit x265 -YTS",
    "sortTitle": "dune part two 2024 1080p webrip 5 1 10bit x265 yts",
    "imdbId": 15239678,
    "tmdbId": 693134,
    "tvdbId": 0,
    "publishDate": "2026-10-11T12:00:00Z",
    "downloadUrl": "http://localhost:8989/1/download?apikey=...&link=...&file=Dune+Part+Two",
    "infoUrl": "https://yts.gg/movies/dune-part-two-2024",
    "indexerFlags": ["freeleech"],
    "categories": [{ "id": 2040, "name": "Movies/HD" }],
    "magnetUrl": "magnet:?xt=urn:btih:...",
    "infoHash": "ABCDEF1234567890ABCDEF1234567890ABCDEF12",
    "seeders": 840,
    "leechers": 45,
    "protocol": "torrent",
    "fileName": "Dune Part Two (2024) 1080p WEBRip 5.1 10Bit x265 -YTS.torrent"
  }
]
```

### 3.4 `/api/v1/applications` (`ApplicationController.cs` & `ApplicationResource.cs`)
Upstream reference: `.tmp/Prowlarr/src/Prowlarr.Api.V1/Applications/ApplicationResource.cs`
* Supports `syncLevel`: `"disabled"`, `"addOnly"`, `"fullSync"`.
* In Replacearr, because Sonarr and Radarr share the unified SQLite `indexers` table directly, `GET /api/v1/applications` returns virtual built-in application entries representing the internal Sonarr and Radarr modules with `"syncLevel": "fullSync"` so monitoring dashboards and users see that TV and Movie modules are natively synchronized.

---

## 4. Torznab & Newznab Protocol (`/{id}/api` & `/api/v1/indexer/{id}/newznab`)

Upstream reference:
* `.tmp/Prowlarr/src/Prowlarr.Api.V1/Indexers/NewznabController.cs`
* `.tmp/Prowlarr/src/NzbDrone.Core/Indexers/IndexerCapabilities.cs`
* `.tmp/Prowlarr/src/NzbDrone.Core/IndexerSearch/NewznabResults.cs`
* `.tmp/Prowlarr/src/NzbDrone.Core/Indexers/NewznabStandardCategory.cs`

### 4.1 Request Parameters (`NewznabRequest.cs`)
* `t` (**required**):
  * `caps`: Returns indexer capabilities XML (`<caps>...</caps>`).
  * `search`: Generic keyword search (`q`, `cat`, `limit`, `offset`).
  * `tvsearch`: TV search (`q`, `season`, `ep`, `tvdbid`, `imdbid`, `tmdbid`, `tvmazeid`, `rid`, `cat`).
  * `movie`: Movie search (`q`, `imdbid`, `tmdbid`, `traktid`, `genre`, `year`, `cat`).
  * `music`: Audio search (`q`, `artist`, `album`, `track`, `label`, `year`, `genre`, `cat`).
  * `book`: Book search (`q`, `title`, `author`, `publisher`, `year`, `genre`, `cat`).
* **Special `id = 0` Behavior (`NewznabController.cs` lines 81–133)**:
  When `id == 0`, Prowlarr returns a synthetic all-categories capability response for `t=caps` and a single `"Test Release"` item for `t=search|tvsearch|movie|music|book`. External *arr applications use `id=0` during Prowlarr Application connection tests!
* **Standard Category IDs (`NewznabStandardCategory.cs`)**:
  * `1000` (`Console`), `2000` (`Movies`: `2010` Foreign, `2030` SD, `2040` HD, `2045` UHD, `2050` BluRay, `2060` 3D, `2070` DVD, `2080` WEB-DL, `2090` x265), `3000` (`Audio`: `3010` MP3, `3020` Video, `3030` Audiobook, `3040` Lossless), `4000` (`PC`), `5000` (`TV`: `5010` WEB-DL, `5020` Foreign, `5030` SD, `5040` HD, `5045` UHD, `5060` Sport, `5070` Anime, `5080` Documentary, `5090` x265), `6000` (`XXX`), `7000` (`Books`: `7010` Mags, `7020` EBook, `7030` Comics), `8000` (`Other`).

### 4.2 Capabilities XML Response (`?t=caps`)
Content-Type: `application/rss+xml`
```xml
<?xml version="1.0" encoding="UTF-8"?>
<caps>
  <server title="Prowlarr" />
  <limits default="100" max="100" />
  <searching>
    <search available="yes" supportedParams="q" />
    <tv-search available="yes" supportedParams="q,season,ep,imdbid,tvdbid,tmdbid" />
    <movie-search available="yes" supportedParams="q,imdbid,tmdbid,year" />
    <music-search available="yes" supportedParams="q,album,artist,track,year" />
    <audio-search available="yes" supportedParams="q,album,artist,track,year" />
    <book-search available="yes" supportedParams="q,title,author,year" />
  </searching>
  <categories>
    <category id="2000" name="Movies">
      <subcat id="2040" name="Movies/HD" />
      <subcat id="2045" name="Movies/UHD" />
    </category>
    <category id="5000" name="TV">
      <subcat id="5040" name="TV/HD" />
      <subcat id="5045" name="TV/UHD" />
    </category>
  </categories>
</caps>
```

### 4.3 Search Feed XML Response (`?t=search|tvsearch|movie`)
Content-Type: `application/rss+xml` (with `xmlns:torznab="http://torznab.com/schemas/2015/feed"` for Torrent indexers or `xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/"` for Usenet indexers):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<rss version="1.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <atom:link rel="self" type="application/rss+xml" />
    <title>Prowlarr</title>
    <item>
      <title>Severance.S01E01.1080p.WEB-DL.DDP5.1.H.264-FLUX</title>
      <guid>https://example.tracker/torrent/1001</guid>
      <prowlarrindexer id="1" type="public">ExampleTracker</prowlarrindexer>
      <comments>https://example.tracker/details/1001</comments>
      <pubDate>Fri, 23 Oct 2026 12:00:00 +0000</pubDate>
      <size>2147483648</size>
      <link>http://localhost:8989/1/download?apikey=KEY&amp;link=...&amp;file=Severance.S01E01</link>
      <category>5040</category>
      <enclosure url="http://localhost:8989/1/download?apikey=KEY&amp;link=...&amp;file=Severance.S01E01" length="2147483648" type="application/x-bittorrent" />
      <torznab:attr name="category" value="5040" />
      <torznab:attr name="seeders" value="150" />
      <torznab:attr name="peers" value="165" />
      <torznab:attr name="tvdbid" value="371980" />
      <torznab:attr name="imdb" value="11280740" />
      <torznab:attr name="infohash" value="ABCDEF1234567890ABCDEF1234567890ABCDEF12" />
      <torznab:attr name="downloadvolumefactor" value="0" />
      <torznab:attr name="uploadvolumefactor" value="1" />
    </item>
  </channel>
</rss>
```
* **Error XML Format (`NewznabController.CreateErrorXML`)**:
  `<error code="200" description="Missing parameter (t)" />` (codes: `100` Incorrect user credentials, `200` Missing parameter, `201` Incorrect parameter, `202` No such function, `410` Indexer disabled, `429` Too many requests).

---

## 5. Cardigann YAML Definition Format (`.tmp/ProwlarrIndexers/definitions/v11/*.yml`)

Upstream references:
* `.tmp/ProwlarrIndexers/definitions/v11/schema.json` (578 `.yml` definitions in `v11/`, plus `v12/`)
* `.tmp/Prowlarr/src/NzbDrone.Core/Indexers/Definitions/Cardigann/CardigannDefinition.cs`
* `.tmp/Prowlarr/src/NzbDrone.Core/Indexers/Definitions/Cardigann/CardigannBase.cs`

### 5.1 Structure of a Cardigann v11 YAML File
Inspected examples (`yts.yml`, `thepiratebay.yml`, `torrentleech.yml`) conform to `definitions/v11/schema.json`:
1. **Metadata Header**:
   * `id` (string, e.g., `yts`, `thepiratebay`, `torrentleech`)
   * `name` (display name)
   * `description` (tracker description)
   * `language` (BCP-47 tag, e.g., `en-US`)
   * `type` (`public`, `semi-private`, or `private`)
   * `encoding` (`UTF-8`, etc.)
   * `requestDelay` (optional float rate-limit in seconds between requests, e.g., `2.5` in `yts.yml`, `4.1` in `torrentleech.yml`)
   * `links` (array of active mirror base URLs) and `legacylinks` (deprecated domains auto-migrated to `links[0]`)
2. **`caps` Block**:
   * `categorymappings`: List of `{ id, cat, desc, default }` mapping tracker-specific category IDs/strings to Newznab category names (`Movies/HD`, `TV/UHD`, `Audio/Lossless`, etc.).
   * `modes`: Supported search modes and parameters:
     * `search: [q]` (mandatory)
     * `tv-search: [q, season, ep, imdbid, tvdbid, tmdbid, tvmazeid, ...]`
     * `movie-search: [q, imdbid, tmdbid, year, genre, ...]`
     * `music-search: [q, album, artist, label, track, year, genre]`
     * `book-search: [q, title, author, publisher, year, genre]`
3. **`settings` Block**:
   * Dynamic user-configurable fields (`type` in `text`, `password`, `checkbox`, `select`, `info`, `info_cookie`, `info_flaresolverr`, `info_useragent`, `info_category_8000`). Exposed in `IndexerResource.Fields` and accessible in templates as `{{ .Config.<name> }}`.
4. **`login` Block** (for `private` / `semi-private` trackers like `torrentleech.yml`):
   * `path`, `submitpath`, `method` (`form`, `post`, `cookie`, `get`, `oneurl`), `form` CSS selector, `inputs` map, `selectorinputs`, `error` selectors, and `test` (`path` + `selector` to verify active session).
5. **`search` Block**:
   * `paths`: List of `SearchPathBlock` entries (`path` URL template, `method`, `inputs`, `categories`, and `response.type`: `html` (default), `json`, or `xml`).
   * `keywordsfilters` & `preprocessingfilters`: Pre-request and pre-parse filter pipelines.
   * `rows`: CSS selector (for HTML/XML via AngleSharp/goquery) or JSONPath selector (for `response.type: json`, e.g., `selector: data.movies`, `attribute: torrents`, `multiple: true` in `yts.yml`), plus optional `count` and `after`.
   * `fields`: Ordered key-value list (`KeyValuePairList` in `CardigannDefinition.cs`, because keys like `category` or `_id` can appear multiple times) extracting `title`, `details`, `download`, `infohash`, `magnet`, `size`, `seeders`, `leechers`, `date`, `imdbid`, `tvdbid`, `tmdbid`, `downloadvolumefactor`, `uploadvolumefactor`, `minimumratio`, `minimumseedtime`. Variables prefixed with `_` (e.g., `_quality`, `_id`, `_filename`) are intermediate variables stored in `.Result.<name>` for use by subsequent fields!
6. **Template Engine & Filter Pipeline (`CardigannBase.cs`)**:
   * **Go-style Templates**: Cardigann definitions natively use Go `text/template` syntax (`{{ .Keywords }}`, `{{ .Query.IMDBID }}`, `{{ .Config.sitelink }}`, `{{ if eq .Result._type "web" }}...{{ else }}...{{ end }}`, `{{ join .Categories "," }}`, `{{ re_replace ... }}`).
   * **25 Built-In Field Filters**: `querystring`, `timeparse`, `dateparse` (which uses Go reference time layout or `yyyy-MM-dd HH:mm:ss`!), `regexp`, `re_replace`, `split`, `replace`, `trim`, `prepend`, `append`, `tolower`, `toupper`, `urldecode`, `urlencode`, `htmldecode`, `htmlencode`, `timeago`, `reltime`, `fuzzytime`, `validfilename`, `diacritics`, `jsonjoinarray`, `hexdump`, `strdump`, `validate`.

### 5.2 How Replacearr Loads Cardigann YAML Definitions Alongside Native Go Plugins
In Replacearr:
1. **Cardigann Loader (`config.DefinitionsDir`)**: On startup, Replacearr scans `REPLACEARR_DEFINITIONS_DIR` (falling back to `.tmp/ProwlarrIndexers/definitions/v11` in development, as configured in `internal/config/config.go`). Because Cardigann was originally invented in Go (`text/template`, Go regexes, Go date layouts), Replacearr parses `.yml` files directly into Go structs and evaluates templates natively without C#'s regex-based Go-template emulation layer.
2. **Native Go Indexer Plugins (`pkg/pluginsdk`)**: Developers can also register compiled or dynamic Go `IndexerPlugin` implementations (`Implementation: "Plugin"`, `PluginID: "..."` in the `indexers` table) for complex APIs, GraphQL trackers, or custom debrid/usenet indexers. Both Cardigann definitions and Go `IndexerPlugin` instances are unified into the same `indexers` table and exposed identically over `/api/v1/indexer`, `/api/v1/search`, and `/{id}/api` Torznab endpoints.
