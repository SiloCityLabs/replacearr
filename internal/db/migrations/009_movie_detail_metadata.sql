-- Movie detail metadata from api.radarr.video (ratings, collection, credits, alts).
ALTER TABLE movies ADD COLUMN original_language TEXT NOT NULL DEFAULT '';
ALTER TABLE movies ADD COLUMN certification TEXT NOT NULL DEFAULT '';
ALTER TABLE movies ADD COLUMN youtube_trailer_id TEXT NOT NULL DEFAULT '';
ALTER TABLE movies ADD COLUMN collection_json TEXT NOT NULL DEFAULT '';
ALTER TABLE movies ADD COLUMN ratings_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE movies ADD COLUMN credits_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE movies ADD COLUMN alternate_titles_json TEXT NOT NULL DEFAULT '[]';
