#!/bin/sh
# Put an uploaded build live (INFRA-003 D3), from api/ on the server:
#
#   docker/geo/switch.sh <date>
#
# 1. Serve <date> from a throwaway Valhalla and route across the region
#    (GEO_CHECK_ROUTE); on failure nothing has changed.
# 2. previous/ is deleted, current/ becomes previous/, <date> becomes current/.
# 3. Recreate `valhalla` on current/ and wait for healthy. The basemap is not
#    here: publish.sh puts it on R2 (INFRA-008); in dev, Caddy reads tiles/ per
#    request, so it flips with the rename.
# 4. Replace geo.places with current/places.geojsonl (GEO_IMPORT).
. "$(dirname "$0")/lib.sh"

DATE=${1:?usage: switch.sh <date>}
is_build "$DATE" && [ -d "$GEO_DATA_DIR/$DATE" ] || { echo "❌ no build $GEO_DATA_DIR/$DATE"; exit 1; }
ensure_tools

echo "🔎 Checking $DATE"
tools sh -ec "cd /data/$DATE
	for f in valhalla/tiles.tar places.geojsonl; do
		[ -s \$f ] || { echo \"❌ $DATE/\$f is missing or empty\"; exit 1; }
	done"
check_valhalla "$DATE" || { echo "❌ $DATE fails the route check — nothing switched"; exit 1; }

echo "🔀 current → previous, $DATE → current"
stop_valhalla
tools sh -ec "cd /data; rm -rf previous; if [ -d current ]; then mv current previous; fi; mv $DATE current"
start_valhalla || { echo "❌ valhalla failed on $DATE — run rollback.sh"; exit 1; }

echo "📍 Importing places"
import_places
echo "✅ $DATE is live; rollback.sh returns to the previous build"
