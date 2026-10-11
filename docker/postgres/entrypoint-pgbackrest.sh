#!/bin/sh
# Writes /etc/pgbackrest/pgbackrest.conf from the BACKUP_S3_* the backup service also reads,
# then hands over to the postgres image's own entrypoint. Every pgbackrest call — the server's
# archive_command, `docker compose exec postgres pgbackrest …`, the drill in docker/restore.sh —
# reads that file. Runs as root (the server) or as postgres (`run -u postgres`); either way
# the file ends up owned by postgres and unreadable to anyone else: it holds the R2 secret.
set -eu

# A one-off with no repository (`docker run --rm <image> pgbackrest version`) needs no file;
# the server does, and refuses to start without one rather than fail every archive_command.
if [ -z "${BACKUP_S3_BUCKET:-}" ] && [ "${1:-postgres}" != postgres ]; then
	exec docker-entrypoint.sh "$@"
fi

: "${BACKUP_S3_BUCKET:?BACKUP_S3_BUCKET is required}"
: "${BACKUP_S3_ACCESS_KEY:?BACKUP_S3_ACCESS_KEY is required}"
: "${BACKUP_S3_SECRET_KEY:?BACKUP_S3_SECRET_KEY is required}"

# pgBackRest wants a host, not the URL aws-cli takes; empty means real AWS S3.
endpoint=${BACKUP_S3_ENDPOINT:-}
endpoint=${endpoint#https://}
endpoint=${endpoint#http://}
endpoint=${endpoint%/}
region=${BACKUP_S3_REGION:-us-east-1}
: "${endpoint:=s3.$region.amazonaws.com}"

conf=/etc/pgbackrest/pgbackrest.conf
umask 077
# R2 has no virtual-host buckets, hence uri-style=path. archive-async pushes WAL from a spool,
# so a slow upload never holds a segment back from recycling; lz4 costs little CPU.
cat > "$conf" <<EOF
[global]
repo1-type=s3
repo1-s3-bucket=$BACKUP_S3_BUCKET
repo1-s3-endpoint=$endpoint
repo1-s3-region=$region
repo1-s3-uri-style=path
repo1-s3-key=$BACKUP_S3_ACCESS_KEY
repo1-s3-key-secret=$BACKUP_S3_SECRET_KEY
repo1-path=/pgbackrest
repo1-retention-full=2
archive-async=y
spool-path=/var/spool/pgbackrest
process-max=2
compress-type=lz4
log-level-console=info
log-level-file=off

[main]
pg1-path=/var/lib/postgresql/data
EOF
[ "$(id -u)" = 0 ] && chown postgres:postgres "$conf"

exec docker-entrypoint.sh "$@"
