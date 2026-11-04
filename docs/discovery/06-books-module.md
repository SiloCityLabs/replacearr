# 06 — Books Module (ebook + audiobook, native-first)

Replacearr manages **ebooks** and **audiobooks** in one library. Metadata comes from **OpenLibrary**. Native `/api/v1` + the SPA Books tab are the product; `/readarr/api/v1` is thin compat only.

## Domain

- `Author` → `Book` (OpenLibrary work) → `Edition` with `format`: `ebook` | `audiobook`
- Foreign IDs are OpenLibrary keys (`/authors/…`, `/works/…`, `/books/…`) plus ISBN when present
- Root folder media types: `ebook` (`/data/Books`), `audiobook` (`/data/Audiobooks`)
- Quality profiles: `ebook` (EPUB/AZW3/MOBI/PDF/CBZ) and `audiobook` (M4B/MP3/FLAC)

## Native API

| Method | Path | Notes |
| :--- | :--- | :--- |
| GET/POST | `/api/v1/authors` | |
| GET | `/api/v1/authors/lookup?term=` | OpenLibrary author search |
| GET | `/api/v1/books?authorId=` | |
| POST | `/api/v1/books` | Body includes `author`, `format`, `rootFolderPath`, `qualityProfileId`; creates monitored edition |
| GET | `/api/v1/books/lookup?term=` | OpenLibrary work/ISBN search (`isbn:`, `ol:` prefixes) |
| GET | `/api/v1/editions?bookId=&format=` | |

Interactive Search / grab accept `authorId` / `bookId` / `editionId`. Torznab `t=book` is advertised; audiobooks prefer audio categories via search mode.

## Thin Readarr compat

Mount: `/readarr/api/v1` (plugin `builtin-compat-readarr`, API key scope `readarr`).

Implemented: `system/status`, `author`, `book`, `edition`, `qualityprofile`, `rootfolder`, `tag`, `queue`, `history`.

## Permissions

`books:read`, `books:write`, `books:delete`
