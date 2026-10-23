-- 002_indexer_proxies.sql: FlareSolverr & Indexer Proxy Configuration

CREATE TABLE IF NOT EXISTS indexer_proxies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    implementation TEXT NOT NULL DEFAULT 'FlareSolverr',
    implementation_name TEXT NOT NULL DEFAULT 'FlareSolverr',
    config_contract TEXT NOT NULL DEFAULT 'FlareSolverrSettings',
    host TEXT NOT NULL DEFAULT 'http://localhost:8191/',
    request_timeout INTEGER NOT NULL DEFAULT 60,
    session_ttl_minutes INTEGER NOT NULL DEFAULT 15,
    disable_media INTEGER NOT NULL DEFAULT 0,
    enable INTEGER NOT NULL DEFAULT 1,
    fields_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
