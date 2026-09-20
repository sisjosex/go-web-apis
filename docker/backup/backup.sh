#!/bin/sh
# Nightly logical backup of every database in the cluster.
#
# One pg_dumpall --globals-only (roles and tablespaces, which a per-database
# dump does not carry) plus one custom-format pg_dump per database, enumerated
# from pg_database at run time — tenants add databases after this container
# started, so the list can never be baked in.
#
# Restore steps: docker/backup/README.md
set -eu

# Present when crond runs us; absent under `docker compose run`.
[ -f /etc/backup.env ] && . /etc/backup.env

: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${BACKUP_S3_BUCKET:?BACKUP_S3_BUCKET is required}"
: "${BACKUP_KEEP_DAYS:=14}"

export PGPASSWORD="$POSTGRES_PASSWORD"

PSQL="psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d postgres -Atq"

# --endpoint-url is what points aws-cli at MinIO, Backblaze, R2 or Spaces.
# Empty (unset) means real AWS S3.
S3_ARGS=""
if [ -n "${BACKUP_S3_ENDPOINT:-}" ]; then
	S3_ARGS="--endpoint-url $BACKUP_S3_ENDPOINT"
fi

DATE=$(date -u +%Y-%m-%d)
DEST="s3://${BACKUP_S3_BUCKET}/${DATE}"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

echo "🗄️  Backup $DATE → $DEST"

echo "  • globals"
pg_dumpall -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" \
	--globals-only > "$WORK/globals.sql"

# datallowconn excludes template0; datistemplate excludes template1. `postgres`
# is the maintenance database and holds nothing of ours.
DATABASES=$($PSQL -c "
	SELECT datname FROM pg_database
	WHERE datallowconn AND NOT datistemplate AND datname <> 'postgres'
	ORDER BY datname
")

for db in $DATABASES; do
	echo "  • $db"
	pg_dump -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" \
		-Fc -d "$db" -f "$WORK/${db}.dump"
done

# shellcheck disable=SC2086 # S3_ARGS is two words or none, on purpose
aws s3 cp "$WORK" "$DEST/" --recursive $S3_ARGS

# Prune. Dates are ISO, so a string comparison is a date comparison. The cutoff
# comes from the server rather than busybox date, whose -d cannot do arithmetic.
CUTOFF=$($PSQL -c "SELECT to_char(now() - interval '$BACKUP_KEEP_DAYS days', 'YYYY-MM-DD')")

# shellcheck disable=SC2086
for prefix in $(aws s3 ls "s3://${BACKUP_S3_BUCKET}/" $S3_ARGS | awk '$1 == "PRE" { print $2 }'); do
	day=${prefix%/}
	case "$day" in
	[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;;
	*) continue ;; # not one of ours
	esac

	if [ "$day" \< "$CUTOFF" ]; then
		echo "  ✂️  pruning $day (older than $BACKUP_KEEP_DAYS days)"
		# shellcheck disable=SC2086
		aws s3 rm "s3://${BACKUP_S3_BUCKET}/${day}/" --recursive $S3_ARGS > /dev/null
	fi
done

echo "✅ Backup $DATE complete"
