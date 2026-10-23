-- 001_init.sql: Replacearr Unified SQLite Schema

CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    permissions_json TEXT NOT NULL DEFAULT '[]',
    built_in INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role_id INTEGER NOT NULL REFERENCES roles(id),
    is_active INTEGER NOT NULL DEFAULT 1,
    last_login_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    raw_key TEXT NOT NULL DEFAULT '',
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope TEXT NOT NULL DEFAULT 'all',
    permissions_json TEXT NOT NULL DEFAULT '[]',
    last_used_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS root_folders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    media_type TEXT NOT NULL DEFAULT 'all',
    accessible INTEGER NOT NULL DEFAULT 1,
    free_space INTEGER NOT NULL DEFAULT 536870912000,
    total_space INTEGER NOT NULL DEFAULT 1073741824000
);

CREATE TABLE IF NOT EXISTS quality_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    upgrade_allowed INTEGER NOT NULL DEFAULT 1,
    cutoff INTEGER NOT NULL DEFAULT 7,
    items_json TEXT NOT NULL DEFAULT '[]',
    min_format_score INTEGER NOT NULL DEFAULT 0,
    cutoff_format_score INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'continuing',
    ended INTEGER NOT NULL DEFAULT 0,
    overview TEXT NOT NULL DEFAULT '',
    network TEXT NOT NULL DEFAULT '',
    air_time TEXT NOT NULL DEFAULT '21:00',
    images_json TEXT NOT NULL DEFAULT '[]',
    seasons_json TEXT NOT NULL DEFAULT '[]',
    year INTEGER NOT NULL DEFAULT 2024,
    path TEXT NOT NULL DEFAULT '',
    quality_profile_id INTEGER NOT NULL DEFAULT 1,
    language_profile_id INTEGER NOT NULL DEFAULT 1,
    season_folder INTEGER NOT NULL DEFAULT 1,
    monitored INTEGER NOT NULL DEFAULT 1,
    use_scene_numbering INTEGER NOT NULL DEFAULT 0,
    runtime INTEGER NOT NULL DEFAULT 45,
    tvdb_id INTEGER NOT NULL DEFAULT 0,
    tv_rage_id INTEGER NOT NULL DEFAULT 0,
    tv_maze_id INTEGER NOT NULL DEFAULT 0,
    imdb_id TEXT NOT NULL DEFAULT '',
    title_slug TEXT NOT NULL DEFAULT '',
    root_folder_path TEXT NOT NULL DEFAULT '/data/tv',
    series_type TEXT NOT NULL DEFAULT 'standard',
    clean_title TEXT NOT NULL DEFAULT '',
    genres_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    tvdb_id INTEGER NOT NULL DEFAULT 0,
    episode_file_id INTEGER NOT NULL DEFAULT 0,
    season_number INTEGER NOT NULL DEFAULT 1,
    episode_number INTEGER NOT NULL DEFAULT 1,
    title TEXT NOT NULL DEFAULT '',
    air_date TEXT NOT NULL DEFAULT '',
    air_date_utc DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    overview TEXT NOT NULL DEFAULT '',
    has_file INTEGER NOT NULL DEFAULT 0,
    monitored INTEGER NOT NULL DEFAULT 1,
    absolute_episode_number INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS movies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    original_title TEXT NOT NULL DEFAULT '',
    sort_title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'released',
    overview TEXT NOT NULL DEFAULT '',
    in_cinemas TEXT NOT NULL DEFAULT '',
    physical_release TEXT NOT NULL DEFAULT '',
    digital_release TEXT NOT NULL DEFAULT '',
    images_json TEXT NOT NULL DEFAULT '[]',
    website TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 2024,
    has_file INTEGER NOT NULL DEFAULT 0,
    movie_file_id INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL DEFAULT '',
    quality_profile_id INTEGER NOT NULL DEFAULT 1,
    monitored INTEGER NOT NULL DEFAULT 1,
    minimum_availability TEXT NOT NULL DEFAULT 'released',
    is_available INTEGER NOT NULL DEFAULT 1,
    folder_name TEXT NOT NULL DEFAULT '',
    runtime INTEGER NOT NULL DEFAULT 120,
    clean_title TEXT NOT NULL DEFAULT '',
    imdb_id TEXT NOT NULL DEFAULT '',
    tmdb_id INTEGER NOT NULL DEFAULT 0,
    title_slug TEXT NOT NULL DEFAULT '',
    root_folder_path TEXT NOT NULL DEFAULT '/data/movies',
    studio TEXT NOT NULL DEFAULT '',
    genres_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    size_on_disk INTEGER NOT NULL DEFAULT 0,
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS indexers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    implementation TEXT NOT NULL DEFAULT 'Torznab',
    implementation_name TEXT NOT NULL DEFAULT 'Torznab',
    config_contract TEXT NOT NULL DEFAULT 'TorznabSettings',
    protocol TEXT NOT NULL DEFAULT 'torrent',
    privacy TEXT NOT NULL DEFAULT 'public',
    enable INTEGER NOT NULL DEFAULT 1,
    enable_rss INTEGER NOT NULL DEFAULT 1,
    enable_automatic_search INTEGER NOT NULL DEFAULT 1,
    enable_interactive_search INTEGER NOT NULL DEFAULT 1,
    supports_rss INTEGER NOT NULL DEFAULT 1,
    supports_search INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 25,
    base_url TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    definition_file TEXT NOT NULL DEFAULT '',
    plugin_id TEXT NOT NULL DEFAULT '',
    categories_json TEXT NOT NULL DEFAULT '[2000,5000]',
    fields_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS download_clients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    implementation TEXT NOT NULL DEFAULT 'qBittorrent',
    implementation_name TEXT NOT NULL DEFAULT 'qBittorrent',
    config_contract TEXT NOT NULL DEFAULT 'QBittorrentSettings',
    protocol TEXT NOT NULL DEFAULT 'torrent',
    enable INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 1,
    host TEXT NOT NULL DEFAULT 'localhost',
    port INTEGER NOT NULL DEFAULT 8080,
    use_ssl INTEGER NOT NULL DEFAULT 0,
    username TEXT NOT NULL DEFAULT 'admin',
    password TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'replacearr',
    remove_completed INTEGER NOT NULL DEFAULT 1,
    remove_failed INTEGER NOT NULL DEFAULT 1,
    fields_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS queue_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL DEFAULT 'tv',
    series_id INTEGER NOT NULL DEFAULT 0,
    episode_id INTEGER NOT NULL DEFAULT 0,
    movie_id INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'downloading',
    tracked_download_status TEXT NOT NULL DEFAULT 'ok',
    tracked_download_state TEXT NOT NULL DEFAULT 'downloading',
    size INTEGER NOT NULL DEFAULT 0,
    sizeleft INTEGER NOT NULL DEFAULT 0,
    timeleft TEXT NOT NULL DEFAULT '00:12:30',
    estimated_completion DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    protocol TEXT NOT NULL DEFAULT 'torrent',
    download_client TEXT NOT NULL DEFAULT 'qBittorrent',
    download_id TEXT NOT NULL DEFAULT '',
    indexer TEXT NOT NULL DEFAULT '',
    output_path TEXT NOT NULL DEFAULT '/data/downloads',
    added DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS history_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL DEFAULT 'tv',
    series_id INTEGER NOT NULL DEFAULT 0,
    episode_id INTEGER NOT NULL DEFAULT 0,
    movie_id INTEGER NOT NULL DEFAULT 0,
    source_title TEXT NOT NULL,
    quality_json TEXT NOT NULL DEFAULT '{}',
    date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event_type TEXT NOT NULL DEFAULT 'grabbed',
    data_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS plugin_configs (
    plugin_id TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 1,
    settings_json TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
