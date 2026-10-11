#!/bin/sh
# Point-in-time recovery on the box, from /srv/taypi24 (INFRA-017). pgBackRest restores the
# whole cluster — every database — from the base backups and WAL in s3://$BACKUP_S3_BUCKET/pgbackrest/
# (docker/backup/README.md → Point-in-time recovery):
#
#   docker/restore.sh info                                   # backups and the restorable window
#   docker/restore.sh drill [--time 'YYYY-MM-DD HH:MM:SS+00'] # restore into pg-drill, time each phase
#   docker/restore.sh drill clean                            # remove pg-drill and its volume
#   docker/restore.sh restore [--time '…'] --yes             # overwrite production's pgdata
#
# No --time means the end of the archived WAL: the last minute or so before the call.
# The drill never touches pgdata: its own container and volume, archiving off.
set -eu
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f docker-compose.prod.yml"
DRILL="$COMPOSE --profile drill"
VOLUME=xanthops_pgdrill
TABLES="SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema')"

usage() {
	sed -n '6,9s/^#   //p' "$0" >&2
	exit 2
}

# psql as the cluster owner inside a service: psql_in <service> -c '…'
psql_in() {
	svc=$1
	shift
	$DRILL exec -T "$svc" sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atq "$@"' sh "$@"
}

# pgbackrest restore in a one-off <service> container: restore_in <service> [flag…]. A --time
# goes in through pgBackRest's own variables (it has a space in it) and promotes on reaching
# the target; without one, every archived segment replays and the server promotes at the end.
restore_in() {
	svc=$1
	shift
	set -- "$svc" pgbackrest --stanza=main "$@" restore
	[ -z "$TIME" ] || set -- -e PGBACKREST_TYPE=time -e "PGBACKREST_TARGET=$TIME" -e PGBACKREST_TARGET_ACTION=promote "$@"
	$DRILL run --rm --no-deps -u postgres "$@"
}

# Polls every 5 s, for up to an hour, until <service> answers <query> with t. A server that
# stops instead (a target it cannot reach) ends the wait with its last log lines.
wait_for() {
	i=0
	until [ "$(psql_in "$1" -c "$2" 2>/dev/null)" = t ]; do
		if [ -z "$($DRILL ps -q --status running "$1")" ]; then
			$DRILL logs --tail 5 "$1" >&2
			echo "❌ $1 stopped before '$2'" >&2
			exit 1
		fi
		i=$((i + 1))
		[ "$i" -gt 720 ] && { echo "❌ $1: no '$2' after 60 min — $COMPOSE logs $1" >&2; exit 1; }
		sleep 5
	done
}

# Recovery to a time stops at the first commit after it; on a quiet server the archive may hold
# none yet, and the restored server then refuses to start. One commit and a WAL switch on the
# live server, then up to 2 min for that segment to reach the archive. Skipped when it is down.
flush_archive() {
	seg=$(psql_in postgres -c 'SELECT txid_current()' -c 'SELECT pg_walfile_name(pg_switch_wal())' 2>/dev/null) \
		|| { echo "postgres is down: the archive ends where it stopped"; return 0; }
	seg=$(echo "$seg" | tail -1)
	i=0
	until [ "$(psql_in postgres -c "SELECT last_archived_wal >= '$seg' FROM pg_stat_archiver")" = t ]; do
		i=$((i + 1))
		[ "$i" -gt 24 ] && { echo "❌ $seg not archived after 2 min — docker/restore.sh info" >&2; exit 1; }
		sleep 5
	done
}

info() {
	$COMPOSE exec -T -u postgres postgres pgbackrest --stanza=main info
	echo "latest archived  $(psql_in postgres -c "SELECT last_archived_time || '  failed ' || failed_count FROM pg_stat_archiver")"
}

drill() {
	docker volume inspect "$VOLUME" >/dev/null 2>&1 \
		&& { echo "❌ $VOLUME exists — docker/restore.sh drill clean" >&2; exit 1; }
	echo "🧪 drill → pg-drill ${TIME:-(end of archive)}"
	flush_archive
	t0=$(date +%s)
	restore_in pg-drill
	t1=$(date +%s)
	$DRILL up -d --no-deps pg-drill
	wait_for pg-drill 'SELECT true'
	t2=$(date +%s)
	wait_for pg-drill 'SELECT NOT pg_is_in_recovery()'
	t3=$(date +%s)
	echo "tables   drill $(psql_in pg-drill -c "$TABLES")  production $(psql_in postgres -c "$TABLES")"
	echo "replayed to $(psql_in pg-drill -c 'SELECT pg_last_wal_replay_lsn()')  size $(psql_in pg-drill -c 'SELECT pg_size_pretty(sum(pg_database_size(datname))) FROM pg_database')"
	echo "⏱️  download $((t1 - t0)) s · start $((t2 - t1)) s · replay $((t3 - t2)) s · total $((t3 - t0)) s"
	echo "   docker/restore.sh drill clean"
}

drill_clean() {
	$DRILL rm -sf pg-drill
	docker volume rm -f "$VOLUME" >/dev/null
	echo "✅ pg-drill and $VOLUME removed"
}

restore() {
	echo "⚠️  Overwrites production's pgdata with the cluster as of ${TIME:-the end of the archive}."
	echo "   Rehearse first: docker/restore.sh drill${TIME:+ --time '$TIME'}"
	[ "$YES" = 1 ] || usage
	t0=$(date +%s)
	# Everything, not only the API: nothing may write between the stop and the restore. Postgres
	# last, after its final writes are in the archive.
	# shellcheck disable=SC2046 # one word per service
	$COMPOSE stop $($COMPOSE ps --services | grep -vx postgres)
	flush_archive
	$COMPOSE stop postgres
	# --delta rewrites only the files that differ from the backup: minutes, not a full download.
	restore_in postgres --delta
	$COMPOSE start postgres
	wait_for postgres 'SELECT NOT pg_is_in_recovery()'
	$COMPOSE up -d --wait --wait-timeout 300
	echo "✅ restored in $(($(date +%s) - t0)) s — the archive continues on a new timeline;"
	echo "   take a full backup now: $COMPOSE exec -T -u postgres postgres pgbackrest --stanza=main --type=full backup"
}

CMD=${1:-}
[ $# -gt 0 ] && shift
TIME=""
YES=0
while [ $# -gt 0 ]; do
	case "$1" in
	--time)
		[ $# -ge 2 ] || usage
		TIME=$2
		shift 2
		;;
	--yes)
		YES=1
		shift
		;;
	clean)
		[ "$CMD" = drill ] || usage
		CMD=drill_clean
		shift
		;;
	*) usage ;;
	esac
done

case "$CMD" in
info) info ;;
drill) drill ;;
drill_clean) drill_clean ;;
restore) restore ;;
*) usage ;;
esac
