CREATE TABLE IF NOT EXISTS series_aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    season_number INTEGER
);

CREATE INDEX IF NOT EXISTS idx_series_aliases_series_id ON series_aliases(series_id);
