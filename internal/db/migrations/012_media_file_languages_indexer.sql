-- 012_media_file_languages_indexer.sql: editable language / indexer metadata on media files

ALTER TABLE movie_files ADD COLUMN languages TEXT NOT NULL DEFAULT '';
ALTER TABLE movie_files ADD COLUMN indexer TEXT NOT NULL DEFAULT '';

ALTER TABLE episode_files ADD COLUMN languages TEXT NOT NULL DEFAULT '';
ALTER TABLE episode_files ADD COLUMN indexer TEXT NOT NULL DEFAULT '';
