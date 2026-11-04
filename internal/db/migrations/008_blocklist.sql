CREATE TABLE IF NOT EXISTS blocklist (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  media_type TEXT NOT NULL,
  source_title TEXT NOT NULL,
  torrent_info_hash TEXT NOT NULL DEFAULT '',
  indexer TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  series_id INTEGER NOT NULL DEFAULT 0,
  movie_id INTEGER NOT NULL DEFAULT 0,
  artist_id INTEGER NOT NULL DEFAULT 0,
  author_id INTEGER NOT NULL DEFAULT 0,
  size INTEGER NOT NULL DEFAULT 0,
  protocol TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_blocklist_media_type ON blocklist(media_type);
CREATE INDEX IF NOT EXISTS idx_blocklist_info_hash ON blocklist(torrent_info_hash);
