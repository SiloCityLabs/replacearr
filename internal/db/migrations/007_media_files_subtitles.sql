-- 007_media_files_subtitles.sql: episode/movie file records + native subtitle tracking

CREATE TABLE IF NOT EXISTS episode_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    path TEXT NOT NULL DEFAULT '',
    relative_path TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    scene_name TEXT NOT NULL DEFAULT '',
    release_group TEXT NOT NULL DEFAULT '',
    date_added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_episode_files_episode ON episode_files(episode_id);
CREATE INDEX IF NOT EXISTS idx_episode_files_series ON episode_files(series_id);

CREATE TABLE IF NOT EXISTS movie_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    path TEXT NOT NULL DEFAULT '',
    relative_path TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    scene_name TEXT NOT NULL DEFAULT '',
    release_group TEXT NOT NULL DEFAULT '',
    date_added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_movie_files_movie ON movie_files(movie_id);

CREATE TABLE IF NOT EXISTS subtitle_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    languages_json TEXT NOT NULL DEFAULT '["en"]',
    prefer_hearing_impaired INTEGER NOT NULL DEFAULT 0,
    include_forced INTEGER NOT NULL DEFAULT 1,
    min_score INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO subtitle_profiles (id, name, languages_json, prefer_hearing_impaired, include_forced, min_score, enabled)
VALUES (1, 'Default', '["en"]', 0, 1, 0, 1);

CREATE TABLE IF NOT EXISTS subtitle_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL DEFAULT 'movie',
    movie_id INTEGER NOT NULL DEFAULT 0,
    episode_id INTEGER NOT NULL DEFAULT 0,
    episode_file_id INTEGER NOT NULL DEFAULT 0,
    movie_file_id INTEGER NOT NULL DEFAULT 0,
    language TEXT NOT NULL DEFAULT 'en',
    path TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    release_name TEXT NOT NULL DEFAULT '',
    score INTEGER NOT NULL DEFAULT 0,
    forced INTEGER NOT NULL DEFAULT 0,
    hearing_impaired INTEGER NOT NULL DEFAULT 0,
    downloaded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_movie ON subtitle_tracks(movie_id);
CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_episode ON subtitle_tracks(episode_id);
