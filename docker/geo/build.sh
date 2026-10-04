#!/bin/sh
# Offline geo build (INFRA-003 D3): one OSM extract → routing tiles, basemap,
# light/dark styles with their fonts and sprites, and the address-search places.
#
# Runs on any machine with Docker — never on the server, whose RAM it would
# exhaust. The result is rsynced there and put live by switch.sh. Downloads are
# cached in $GEO_DATA_DIR/sources; the Geofabrik extracts refresh when upstream
# is newer. A failed build leaves only <date>.partial behind, never a <date> dir.
#
#   docker/geo/build.sh [<date>]      # default: today, UTC
#
# Output: $GEO_DATA_DIR/<date>/
#   valhalla/   valhalla.json, tiles.tar, admins.sqlite
#   tiles/      basemap.pmtiles, style-{light,dark}.json, fonts/, sprites/
#   places.geojsonl
#   BUILD       one line: date, regions, pinned refs, TILES_URL — what current/ is
. "$(dirname "$0")/lib.sh"

DATE=${1:-$(date -u +%Y-%m-%d)}
is_build "$DATE" || { echo "❌ '$DATE' is not a YYYY-MM-DD date"; exit 1; }

: "${GEO_REGIONS:=south-america/bolivia}"   # Geofabrik paths, comma-separated (D1)
: "${TILES_URL:=http://127.0.0.1:8090/tiles}" # baked into the styles; 127.0.0.1, not localhost (AGENTS.md)
: "${GEO_STYLE_LANG:=es}"                     # label language, falls back to `name`
: "${GEO_BUILD_MEMORY:=3g}"                   # JVM heap for the basemap

BASEMAP_REF=ca93fc06efbaff1c5f069d06edbe5de18839c1c8 # protomaps/basemaps, tiles 4.15.2
BASEMAP_IMAGE=taypi-geo-basemap:4.15.2
ASSETS_REF=028c18f713baecad011301ff7a69acc39bcc2ae7  # protomaps/basemaps-assets

[ -e "$GEO_DATA_DIR/$DATE" ] && { echo "❌ $GEO_DATA_DIR/$DATE already exists"; exit 1; }
WORK=$DATE.partial
LOG=$GEO_DATA_DIR/$WORK.log

# stage <name> <cmd...> — quiet; the log tail on failure.
stage() {
	echo "  • $1"
	shift
	"$@" > "$LOG" 2>&1 || { tail -20 "$LOG"; echo "❌ build failed — $LOG"; exit 1; }
}

echo "🗺️  Geo build $DATE — $GEO_REGIONS"
ensure_tools
tools sh -c "rm -rf /data/$WORK && mkdir -p /data/sources /data/$WORK/valhalla /data/$WORK/tiles"

PBFS=""
for region in $(echo "$GEO_REGIONS" | tr ',' ' '); do
	file=$(echo "$region" | tr '/' '_').osm.pbf
	stage "extract $region" tools sh -ec "cd /data/sources
		if [ -f '$file' ]; then z='-z $file'; else z=''; fi
		curl -fsSLR \$z -o '$file' 'https://download.geofabrik.de/$region-latest.osm.pbf'"
	PBFS="$PBFS /data/sources/$file"
done
# One region makes this a copy; several become the one extract every stage reads.
# shellcheck disable=SC2086
stage "merge" tools osmium merge --overwrite --no-progress -o "/data/$WORK/region.osm.pbf" $PBFS

# The matrix limit is raised from 2500 pairs (~50 points) to 40 000 (~200): VROOM asks one matrix
# over every rider of a proposal and the destination, and a proposal takes at most 100 riders — a
# 500-point bus matrix already exhausts the container's 1 GB and restarts it (TRACK-014).
# valhalla.json holds the paths the serving container mounts (/valhalla). The
# build itself runs on the container's own disk — its memory-mapped scratch
# files segfault on a Docker Desktop bind mount — and only the results are
# copied out.
stage "routing tiles" docker run --rm -v "$DATA/$WORK:/work" -v "$DATA/$WORK/valhalla:/valhalla" \
	"$VALHALLA_IMAGE" sh -ec '
	valhalla_build_config \
		--mjolnir-tile-dir /valhalla/tiles --mjolnir-tile-extract /valhalla/tiles.tar \
		--mjolnir-admin /valhalla/admins.sqlite --mjolnir-timezone "" \
		--mjolnir-traffic-extract "" --additional-data-elevation "" \
		--loki-logging-level warn --thor-logging-level warn \
		--odin-logging-level warn --meili-logging-level warn \
		--httpd-service-listen "tcp://*:8002" \
		--service-limits-bus-max-matrix-location-pairs 40000 \
		--service-limits-auto-max-matrix-location-pairs 40000 > /valhalla/valhalla.json
	mkdir /build
	sed "s|/valhalla/|/build/|g" /valhalla/valhalla.json > /build/valhalla.json
	valhalla_build_admins -c /build/valhalla.json /work/region.osm.pbf
	valhalla_build_tiles -c /build/valhalla.json /work/region.osm.pbf
	valhalla_build_extract -c /build/valhalla.json -v
	cp /build/tiles.tar /build/admins.sqlite /valhalla/'

docker image inspect "$BASEMAP_IMAGE" > /dev/null 2>&1 \
	|| stage "basemap image" docker build -t "$BASEMAP_IMAGE" "https://github.com/protomaps/basemaps.git#$BASEMAP_REF:tiles"
# --download fetches only the sources missing from data/sources (water, land,
# landcover, Natural Earth — a few GB, once). Same container-disk rule as above.
stage "basemap" docker run --rm -e "JAVA_TOOL_OPTIONS=-Xmx$GEO_BUILD_MEMORY" \
	-v "$DATA/sources:/tiles/data/sources" -v "$DATA/$WORK:/work" \
	--entrypoint sh "$BASEMAP_IMAGE" -ec '
	java -jar /tiles/protomaps-basemap.jar --download --osm_path=/work/region.osm.pbf \
		--tmpdir=/tmp/planetiler --output=/tmp/basemap.pmtiles --force
	cp /tmp/basemap.pmtiles /work/tiles/'

stage "styles, fonts, sprites" tools sh -ec "
	assets=/data/sources/basemaps-assets-$ASSETS_REF.tar.gz
	[ -f \$assets ] || curl -fsSLR -o \$assets https://github.com/protomaps/basemaps-assets/archive/$ASSETS_REF.tar.gz
	cd /data/$WORK/tiles
	node /opt/geo/style.mjs '$TILES_URL' '$GEO_STYLE_LANG' . > /tmp/wanted
	top=basemaps-assets-$ASSETS_REF
	tar -tzf \$assets | sed -n \"s|^\$top/fonts/\([^/]*\)/\$|\1|p\" > /tmp/stacks
	grep -Fxf /tmp/wanted /tmp/stacks | sed \"s|^|\$top/fonts/|\" > /tmp/members
	echo \$top/sprites/v4/ >> /tmp/members
	tar -xzf \$assets --strip-components=1 -T /tmp/members"

stage "places" tools sh -ec "cd /data/$WORK
	osmium tags-filter --no-progress -O -o named.osm.pbf region.osm.pbf nwr/name
	osmium tags-filter --no-progress -O -o kinds.osm.pbf named.osm.pbf nwr/highway nwr/place nwr/amenity nwr/shop
	osmium export --no-progress -O -c /opt/geo/places.json -f geojsonseq -x print_record_separator=false \
		-o places.geojsonl kinds.osm.pbf
	rm named.osm.pbf kinds.osm.pbf region.osm.pbf
	echo '$DATE $GEO_REGIONS basemap=$BASEMAP_REF assets=$ASSETS_REF tiles_url=$TILES_URL' > BUILD
	mv /data/$WORK /data/$DATE"

rm -f "$LOG"
echo "✅ $GEO_DATA_DIR/$DATE — upload it beside current/, then switch.sh $DATE"
