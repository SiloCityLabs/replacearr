-- 011_import_list_exclusions.sql: block titles from being re-added by import lists

CREATE TABLE IF NOT EXISTS import_list_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL DEFAULT 'movie',
    tmdb_id INTEGER NOT NULL DEFAULT 0,
    tvdb_id INTEGER NOT NULL DEFAULT 0,
    imdb_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_import_list_exclusions_media ON import_list_exclusions(media_type);
CREATE INDEX IF NOT EXISTS idx_import_list_exclusions_tmdb ON import_list_exclusions(tmdb_id);
CREATE INDEX IF NOT EXISTS idx_import_list_exclusions_tvdb ON import_list_exclusions(tvdb_id);
