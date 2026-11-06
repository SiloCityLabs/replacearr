-- Per-indexer FlareSolverr opt-in. Existing rows default to enabled so prior
-- global FlareSolverr proxy behavior is preserved; new rows set the flag from the UI.
ALTER TABLE indexers ADD COLUMN use_flare_solverr INTEGER NOT NULL DEFAULT 1;
