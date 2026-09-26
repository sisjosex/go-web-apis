#!/bin/sh
# Swap current/ and previous/ (INFRA-003 D3), from api/ on the server:
#
#   docker/geo/rollback.sh
#
# Running it twice returns to where it started. Places are re-imported, since
# geo.places always mirrors current/.
. "$(dirname "$0")/lib.sh"

[ -d "$GEO_DATA_DIR/previous" ] || { echo "❌ no previous build in $GEO_DATA_DIR"; exit 1; }
ensure_tools

echo "↩️  current ⇄ previous"
stop_valhalla
tools sh -ec "cd /data; mv current rollback.tmp; mv previous current; mv rollback.tmp previous"
start_valhalla

echo "📍 Importing places"
import_places
echo "✅ Rolled back"
