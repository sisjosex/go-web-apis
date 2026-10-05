-- Query statistics: docker-compose.prod.yml preloads the library, this exposes the view.
-- Elsewhere (no preload) the extension still installs; only reading the view fails.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
