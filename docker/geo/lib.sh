# Shared by build.sh, switch.sh and rollback.sh — sourced, never run.
#
# Everything that writes under $GEO_DATA_DIR runs in a container as root, so a
# Linux host needs nothing but Docker and the scripts behave the same from Git
# Bash on Windows.
set -eu

# Git Bash would rewrite /data in `-v host:/data` into a Windows path.
export MSYS_NO_PATHCONV=1

GEO_DIR=$(cd "$(dirname "$0")" && pwd)
: "${GEO_DATA_DIR:=$GEO_DIR/data}"
mkdir -p "$GEO_DATA_DIR"
# Docker Desktop wants D:/..., which `pwd -W` gives under Git Bash; plain pwd elsewhere.
DATA=$(cd "$GEO_DATA_DIR" && (pwd -W 2>/dev/null || pwd))

# The compose file whose `valhalla` serves current/ — the dev one from the Makefile.
: "${GEO_COMPOSE:=docker-compose.prod.yml}"

VALHALLA_IMAGE=ghcr.io/valhalla/valhalla:3.9.0
TOOLS_IMAGE=taypi-geo-tools:1

# tools <cmd...> — run in the tools image with the data dir at /data.
tools() {
	docker run --rm -v "$DATA:/data" "$TOOLS_IMAGE" "$@"
}

ensure_tools() {
	docker image inspect "$TOOLS_IMAGE" > /dev/null 2>&1 || docker build -q -t "$TOOLS_IMAGE" "$GEO_DIR" > /dev/null
}

# A build dir is a UTC date; `current` and `previous` are build dirs renamed in place.
is_build() {
	case "$1" in
	[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) return 0 ;;
	*) return 1 ;;
	esac
}

# check_valhalla <dir-under-data> — serve that build from a throwaway container and
# ask it for a real route; the serving stack is not touched.
check_valhalla() {
	name=taypi-geo-check
	docker rm -f "$name" > /dev/null 2>&1 || true
	docker run -d --name "$name" -v "$DATA/$1/valhalla:/valhalla:ro" "$VALHALLA_IMAGE" \
		valhalla_service /valhalla/valhalla.json 1 > /dev/null
	ok=1
	for _ in $(seq 1 30); do
		if docker exec "$name" curl -fsS -o /dev/null -d "$(route_json)" http://localhost:8002/route; then
			ok=0
			break
		fi
		sleep 2
	done
	[ $ok -eq 0 ] || docker logs --tail 20 "$name"
	docker rm -f "$name" > /dev/null
	return $ok
}

# The health route: two points inside the built region, "lat,lng;lat,lng".
# The default crosses central Cochabamba (INFRA-003 D1).
route_json() {
	: "${GEO_CHECK_ROUTE:=-17.3935,-66.1570;-17.3700,-66.1450}"
	from=${GEO_CHECK_ROUTE%;*}
	to=${GEO_CHECK_ROUTE#*;}
	printf '{"locations":[{"lat":%s,"lon":%s},{"lat":%s,"lon":%s}],"costing":"bus"}' \
		"${from%,*}" "${from#*,}" "${to%,*}" "${to#*,}"
}

# stop_valhalla — before renaming current/: Docker Desktop cannot rename a
# directory whose tiles.tar a running container holds open. Routing is down
# until start_valhalla; /geo/eta falls back meanwhile (D5).
stop_valhalla() {
	docker compose -f "$GEO_COMPOSE" stop valhalla
}

# start_valhalla — recreate the serving valhalla on whatever current/ now holds
# (a new container re-resolves the bind mount) and wait for its healthcheck.
start_valhalla() {
	docker compose -f "$GEO_COMPOSE" up -d --force-recreate --no-deps valhalla
	for _ in $(seq 1 60); do
		state=$(docker inspect -f '{{.State.Health.Status}}' "$(docker compose -f "$GEO_COMPOSE" ps -q valhalla)")
		[ "$state" = healthy ] && { echo "  valhalla healthy"; return 0; }
		sleep 2
	done
	echo "❌ valhalla not healthy after 120 s"
	return 1
}

# import_places — load current/places.geojsonl into geo.places. GEO_IMPORT is the
# whole command; the default runs the cli in the prod image as the `migrate` job
# (the platform database, where the server reads geo.places), with the data dir
# mounted at /geo.
import_places() {
	: "${GEO_IMPORT:=docker compose -f docker-compose.prod.yml run --rm -v $DATA:/geo:ro migrate ./cli geo import /geo/current/places.geojsonl}"
	# shellcheck disable=SC2086 # a command line, split on purpose
	$GEO_IMPORT
}
