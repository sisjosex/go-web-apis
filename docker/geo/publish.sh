#!/bin/sh
# Publish a build's basemap to the R2 bucket behind tiles.taypi24.com (INFRA-008), from api/ on
# the build machine — never on the server:
#
#   docker/geo/publish.sh <date>       # rollback: publish.sh <previous date>
#
# 1. Refuse when <date>/tiles/basemap.pmtiles exceeds Cloudflare's cacheable size (D2).
# 2. Read the live date from the bucket's root style-light.json.
# 3. Sync <date>/tiles/ to s3://$TILES_S3_BUCKET/<date>/ — immutable, cached 30 days. A dated
#    prefix never changes, so a rollback's sync uploads nothing.
# 4. Regenerate both styles for https://<host>/<date> and upload them to the root, last, cached
#    5 min: until then every client keeps reading the previous build whole, never a mix (D1).
# 5. Delete every dated prefix except <date> and the one that was live — open maps keep reading
#    the latter until they reload their style.
. "$(dirname "$0")/lib.sh"

DATE=${1:?usage: publish.sh <date>}
is_build "$DATE" || { echo "❌ '$DATE' is not a YYYY-MM-DD date"; exit 1; }
# The build is <date>/, or current/ or previous/ once a local switch.sh renamed it (BUILD says which).
SRC=$DATE
for dir in current previous; do
	if [ ! -d "$GEO_DATA_DIR/$SRC" ] && [ "$(cut -d' ' -f1 "$GEO_DATA_DIR/$dir/BUILD" 2> /dev/null)" = "$DATE" ]; then
		SRC=$dir
	fi
done
[ -s "$GEO_DATA_DIR/$SRC/tiles/basemap.pmtiles" ] || { echo "❌ no basemap for $DATE in $GEO_DATA_DIR"; exit 1; }

: "${TILES_S3_ENDPOINT:?TILES_S3_ENDPOINT is required (https://<account>.r2.cloudflarestorage.com)}"
: "${TILES_S3_BUCKET:=tiles}"
: "${TILES_S3_ACCESS_KEY:?TILES_S3_ACCESS_KEY is required}"
: "${TILES_S3_SECRET_KEY:?TILES_S3_SECRET_KEY is required}"
: "${TILES_URL:=https://tiles.taypi24.com}"   # the bucket's custom domain; the styles live at its root
: "${GEO_STYLE_LANG:=es}"
: "${TILES_MAX_BYTES:=536870912}"             # 512 MiB, the largest file Cloudflare's free plan caches

size=$(wc -c < "$GEO_DATA_DIR/$SRC/tiles/basemap.pmtiles")
if [ "$size" -gt "$TILES_MAX_BYTES" ]; then
	echo "❌ basemap.pmtiles is $size B, over $TILES_MAX_BYTES B: Cloudflare would not cache it."
	echo "   Rebuild with a lower max zoom (docker/geo/README.md → Basemap size)."
	exit 1
fi

AWS_IMAGE=amazon/aws-cli:2.22.35
# s3 <args...> — aws s3 against the bucket's endpoint, the data dir at /data. Checksums only
# when required: R2 rejects some of the defaults newer aws-cli versions send.
s3() {
	docker run --rm -i -v "$DATA:/data" \
		-e AWS_ACCESS_KEY_ID="$TILES_S3_ACCESS_KEY" -e AWS_SECRET_ACCESS_KEY="$TILES_S3_SECRET_KEY" \
		-e AWS_DEFAULT_REGION=auto -e AWS_REQUEST_CHECKSUM_CALCULATION=when_required \
		-e AWS_RESPONSE_CHECKSUM_VALIDATION=when_required \
		"$AWS_IMAGE" --endpoint-url "$TILES_S3_ENDPOINT" s3 "$@"
}
BUCKET=s3://$TILES_S3_BUCKET

# Empty on the first publish.
LIVE=$(s3 cp "$BUCKET/style-light.json" - 2> /dev/null \
	| grep -o '[0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}/basemap\.pmtiles' | head -1 | cut -d/ -f1) || true
echo "🗺️  Publish $DATE to $BUCKET (live: ${LIVE:-none})"

echo "  • sync $SRC/tiles/"
# The build's own styles name the URL it was built with, never the dated one.
s3 sync "/data/$SRC/tiles/" "$BUCKET/$DATE/" --only-show-errors --exclude 'style-*.json' \
	--cache-control "public, max-age=2592000, immutable"

echo "  • styles → $TILES_URL/$DATE"
ensure_tools
OUT=$DATE.styles
tools sh -ec "rm -rf /data/$OUT && mkdir /data/$OUT
	node /opt/geo/style.mjs '$TILES_URL/$DATE' '$GEO_STYLE_LANG' /data/$OUT > /dev/null"
for flavor in light dark; do
	s3 cp "/data/$OUT/style-$flavor.json" "$BUCKET/style-$flavor.json" --only-show-errors \
		--content-type application/json --cache-control "public, max-age=300"
done
tools rm -rf "/data/$OUT"

# Republishing the live date changes nothing, so it keeps the rollback target too.
[ "$DATE" = "$LIVE" ] || for prefix in $(s3 ls "$BUCKET/" | awk '$1 == "PRE" { print $2 }'); do
	day=${prefix%/}
	is_build "$day" || continue # not one of ours
	[ "$day" = "$DATE" ] || [ "$day" = "$LIVE" ] && continue
	echo "  ✂️  pruning $day"
	s3 rm "$BUCKET/$day/" --recursive --only-show-errors
done

echo "✅ $DATE is live at $TILES_URL/style-{light,dark}.json for every map that loads its style from now; open maps keep the previous one until they reload"
