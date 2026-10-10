#!/bin/sh
# Turn the costly observability pieces on and off on the box, from /srv/taypi24
# (INFRA-016). Prometheus, postgres-exporter, pg_stat_statements and the 500 ms
# slow-query line stay on all the time; these two are for an investigation:
#
#   docker/observe.sh                    # status
#   docker/observe.sh grafana on|off     # dashboards on 127.0.0.1:3001 (~150 MB while up)
#   docker/observe.sh querylog on|off    # every statement in `docker compose logs postgres`, or ≥500 ms only
#
# querylog goes through ALTER SYSTEM + pg_reload_conf(): no restart, no dropped
# connection, and it survives one. Its disk is capped by the json-file logging (5 × 20 MB).
set -eu
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f docker-compose.prod.yml --profile metrics"

# psql inside the postgres container, as the cluster owner; one -c per statement
# because ALTER SYSTEM refuses to run inside the implicit transaction of a multi-statement -c.
psql_c() {
	$COMPOSE exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atq "$@"' sh "$@"
}

querylog() {
	psql_c -c "ALTER SYSTEM SET log_min_duration_statement = '$1'" -c "SELECT pg_reload_conf()" >/dev/null
}

status() {
	echo "grafana   $($COMPOSE ps -a --format '{{.Status}}' grafana | grep . || echo 'not created')"
	echo "querylog  log_min_duration_statement = $(psql_c -c 'SHOW log_min_duration_statement')"
}

case "${1:-status} ${2:-}" in
"grafana on") $COMPOSE up -d --wait grafana ;;
"grafana off") $COMPOSE stop grafana ;;
"querylog on") querylog 0 ;;
"querylog off") querylog 500ms ;;
"status ") ;;
*)
	echo "usage: $0 [grafana on|off | querylog on|off]" >&2
	exit 2
	;;
esac
status
