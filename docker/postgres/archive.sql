-- WAL archiving to R2 through pgBackRest (INFRA-017). Baked into the production image, not
-- in init.sql: the local compose mounts init.sql into the plain postgis image, which has no
-- pgbackrest, and a failing archive_command keeps every WAL segment on disk. archive_mode
-- takes effect at server start; an existing volume never re-runs this file, so there the
-- three lines go in by hand (docker/README.md → Backups).
ALTER SYSTEM SET archive_mode = on;
ALTER SYSTEM SET archive_command = 'pgbackrest --stanza=main archive-push %p';
-- A segment closes and uploads at least once a minute on a quiet night: RPO ≈ 1-2 min.
ALTER SYSTEM SET archive_timeout = '60s';
