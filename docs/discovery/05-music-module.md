# 05 — Music Module (native-first)

Replacearr ships a first-class **Music** library powered by **MusicBrainz** (WS2) and Cover Art Archive. The product surface is native `/api/v1` + the SPA Music tab. `/lidarr/api/v1` is a thin compatibility mount for migration tools only.

## Domain

- `Artist` → `Album` (release-group) → `Track`
- Foreign IDs are MusicBrainz UUIDs (`foreignArtistId`, `foreignAlbumId`, `foreignTrackId`)
- Root folder media type: `music` (default path `/data/Music`)
- Quality profile media type: `music` (MP3 / AAC / FLAC ladder)

## Native API

| Method | Path | Notes |
| :--- | :--- | :--- |
| GET/POST | `/api/v1/artists` | Library CRUD; POST syncs discography from MusicBrainz |
| GET | `/api/v1/artists/lookup?term=` | MusicBrainz search (`mbid:` prefix supported) |
| DELETE | `/api/v1/artists/{id}` | Cascades albums/tracks |
| GET | `/api/v1/albums?artistId=` | |
| GET | `/api/v1/tracks?albumId=` / `artistId=` | |

Interactive Search / grab accept `artistId` / `albumId` and `mediaType=music`. Torznab `t=music` is advertised on Prowlarr caps.

## Thin Lidarr compat

Mount: `/lidarr/api/v1` (plugin `builtin-compat-lidarr`, API key scope `lidarr`).

Implemented: `system/status`, `artist`, `album`, `track`, `qualityprofile`, `rootfolder`, `tag`, `queue`, `history`.

Not chased: MetadataProfile, commands, full Lidarr JSON quirks.

## Permissions

`music:read`, `music:write`, `music:delete`
