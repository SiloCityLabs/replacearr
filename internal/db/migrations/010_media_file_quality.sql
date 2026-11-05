-- Persist detected quality / resolution / codecs for library media files.
ALTER TABLE movie_files ADD COLUMN quality TEXT NOT NULL DEFAULT '';
ALTER TABLE movie_files ADD COLUMN resolution INTEGER NOT NULL DEFAULT 0;
ALTER TABLE movie_files ADD COLUMN video_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE movie_files ADD COLUMN audio_codec TEXT NOT NULL DEFAULT '';

ALTER TABLE episode_files ADD COLUMN quality TEXT NOT NULL DEFAULT '';
ALTER TABLE episode_files ADD COLUMN resolution INTEGER NOT NULL DEFAULT 0;
ALTER TABLE episode_files ADD COLUMN video_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE episode_files ADD COLUMN audio_codec TEXT NOT NULL DEFAULT '';
