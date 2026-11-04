-- 006_music_books.sql: Music (Lidarr-like) and Books (ebook + audiobook) domain tables

ALTER TABLE quality_profiles ADD COLUMN media_type TEXT NOT NULL DEFAULT 'video';

ALTER TABLE queue_items ADD COLUMN artist_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE queue_items ADD COLUMN album_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE queue_items ADD COLUMN author_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE queue_items ADD COLUMN book_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE queue_items ADD COLUMN edition_id INTEGER NOT NULL DEFAULT 0;

ALTER TABLE history_records ADD COLUMN artist_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE history_records ADD COLUMN album_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE history_records ADD COLUMN author_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE history_records ADD COLUMN book_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE history_records ADD COLUMN edition_id INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS artists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'continuing',
    overview TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    path TEXT NOT NULL DEFAULT '',
    quality_profile_id INTEGER NOT NULL DEFAULT 1,
    monitored INTEGER NOT NULL DEFAULT 1,
    foreign_artist_id TEXT NOT NULL DEFAULT '',
    title_slug TEXT NOT NULL DEFAULT '',
    clean_title TEXT NOT NULL DEFAULT '',
    root_folder_path TEXT NOT NULL DEFAULT '/data/Music',
    genres_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    disambiguation TEXT NOT NULL DEFAULT '',
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_artists_foreign_id ON artists(foreign_artist_id) WHERE foreign_artist_id != '';

CREATE TABLE IF NOT EXISTS albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    artist_id INTEGER NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    foreign_album_id TEXT NOT NULL DEFAULT '',
    release_date TEXT NOT NULL DEFAULT '',
    album_type TEXT NOT NULL DEFAULT 'Album',
    overview TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    monitored INTEGER NOT NULL DEFAULT 1,
    has_file INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    genres_json TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums(artist_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_albums_foreign_id ON albums(foreign_album_id) WHERE foreign_album_id != '';

CREATE TABLE IF NOT EXISTS tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    artist_id INTEGER NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    foreign_track_id TEXT NOT NULL DEFAULT '',
    track_number INTEGER NOT NULL DEFAULT 1,
    absolute_track_number INTEGER NOT NULL DEFAULT 1,
    title TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    explicit INTEGER NOT NULL DEFAULT 0,
    has_file INTEGER NOT NULL DEFAULT 0,
    monitored INTEGER NOT NULL DEFAULT 1,
    medium_number INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album_id);
CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist_id);

CREATE TABLE IF NOT EXISTS authors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'continuing',
    overview TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    path TEXT NOT NULL DEFAULT '',
    quality_profile_id INTEGER NOT NULL DEFAULT 1,
    monitored INTEGER NOT NULL DEFAULT 1,
    foreign_author_id TEXT NOT NULL DEFAULT '',
    title_slug TEXT NOT NULL DEFAULT '',
    clean_title TEXT NOT NULL DEFAULT '',
    root_folder_path TEXT NOT NULL DEFAULT '/data/Books',
    genres_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_authors_foreign_id ON authors(foreign_author_id) WHERE foreign_author_id != '';

CREATE TABLE IF NOT EXISTS books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id INTEGER NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    foreign_book_id TEXT NOT NULL DEFAULT '',
    title_slug TEXT NOT NULL DEFAULT '',
    release_date TEXT NOT NULL DEFAULT '',
    overview TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    monitored INTEGER NOT NULL DEFAULT 1,
    series_title TEXT NOT NULL DEFAULT '',
    series_position TEXT NOT NULL DEFAULT '',
    genres_json TEXT NOT NULL DEFAULT '[]',
    page_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_books_author ON books(author_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_books_foreign_id ON books(foreign_book_id) WHERE foreign_book_id != '';

CREATE TABLE IF NOT EXISTS editions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    foreign_edition_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT 'ebook',
    isbn13 TEXT NOT NULL DEFAULT '',
    isbn10 TEXT NOT NULL DEFAULT '',
    asin TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT 'eng',
    page_count INTEGER NOT NULL DEFAULT 0,
    monitored INTEGER NOT NULL DEFAULT 1,
    has_file INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL DEFAULT '',
    quality_profile_id INTEGER NOT NULL DEFAULT 1,
    root_folder_path TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    publisher TEXT NOT NULL DEFAULT '',
    release_date TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_editions_book ON editions(book_id);
CREATE INDEX IF NOT EXISTS idx_editions_author ON editions(author_id);
CREATE INDEX IF NOT EXISTS idx_editions_format ON editions(format);
