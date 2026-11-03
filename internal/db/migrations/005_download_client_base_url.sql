-- Store download client connection as a single base URL (scheme, host, port, path).
ALTER TABLE download_clients ADD COLUMN base_url TEXT NOT NULL DEFAULT '';

UPDATE download_clients
SET base_url = CASE
    WHEN use_ssl = 1 THEN 'https://' || host || ':' || CAST(port AS TEXT)
    ELSE 'http://' || host || ':' || CAST(port AS TEXT)
END
WHERE TRIM(base_url) = '';
