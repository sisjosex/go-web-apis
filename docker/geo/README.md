# Geo stack — routing, basemap, address search (INFRA-003, INFRA-008)

One OpenStreetMap extract feeds three things:

| Piece | Built into | Served by |
|---|---|---|
| Routing tiles | `valhalla/tiles.tar` | the `valhalla` service on the box (`/geo/route`, `/geo/eta` in the API) |
| Basemap, styles, fonts, sprites | `tiles/` | R2 bucket `tiles` behind Cloudflare's cache at `https://tiles.taypi24.com/` (dev: Caddy, `http://127.0.0.1:8090/tiles/`) |
| Named streets, places, POIs | `places.geojsonl` | `geo.places` in the platform database (`/geo/geocode`, `/geo/reverse`) |

The server never builds: a build peaks at 2-4 GB of RAM and downloads a few GB. It runs on any
machine with Docker (the PC); the basemap goes from there to R2, routing and places to the box.
The box never holds, uploads or serves the basemap.

## Layout of `$GEO_DATA_DIR`

```
sources/        download cache: Geofabrik extracts, water/land polygons, Natural Earth,
                landcover, fonts (~2.5 GB, reused by every build)
2026-10-01/     a finished build, not live yet
current/        what valhalla and geo.places serve (and, in dev, Caddy)
previous/       the build before it — rollback.sh swaps the two
```

`current/BUILD` says which build is live: date, regions, pinned refs, the `TILES_URL` baked in.
On the box the builds hold `valhalla/` and `places.geojsonl` only: `tiles/` is never uploaded.

## R2 bucket layout

```
style-light.json, style-dark.json   point at the live date        Cache-Control: max-age=300
2026-10-01/basemap.pmtiles, fonts/, sprites/   the live build     max-age=2592000, immutable
2026-09-01/…                        the build before it — maps opened before the switch read it
```

Every client (MapView in the app, the mobile apps) only ever loads `<TILES_URL>/style-{light,dark}.json`.
The style names one dated folder, so a map reads one build whole, never a mix.

## Updating the maps (monthly)

Three rules keep an update from blocking the box, saturating its bandwidth or showing an
inconsistent map:

- **The box does no heavy work.** Building runs on the PC; the basemap (503 MiB) goes from the PC
  straight to R2. The box receives ~360 MB of routing and places, and switches.
- **Nothing is overwritten in place.** Each build lives under its own date, on R2 and on the box.
  Going live is a pointer change: the two root styles on R2, the `current/` rename on the box.
  Half an upload is never live, and an interrupted run is simply run again.
- **A published date is never republished with different content.** Its files are cached for 30
  days at Cloudflare and in browsers; new content under the same URLs would mix two builds in one
  map. A new build always takes a new date (`build.sh` refuses an existing one).

On the PC, from `api/` (the `TILES_S3_*` variables of `.env.example` in the shell):

```bash
# 1. Build — ~15 min warm, longer the first time
GEO_REGIONS=south-america/bolivia TILES_URL=https://tiles.taypi24.com docker/geo/build.sh 2026-10-01

# 2. Routing and places to the box, throttled so the API keeps its bandwidth; resumable
rsync -a --partial --bwlimit=5000 --exclude tiles/ docker/geo/data/2026-10-01 root@<box>:/srv/geo/
```

On the box, off-hours, from the deploy checkout (`GEO_DATA_DIR=/srv/geo` in `.env`):

```bash
# 3. Check routing on the new build, then switch it live
GEO_DATA_DIR=/srv/geo docker/geo/switch.sh 2026-10-01
```

Back on the PC, once `switch.sh` passed:

```bash
# 4. Basemap to R2
docker/geo/publish.sh 2026-10-01
```

Why this order: `switch.sh` is the only step that tests the build (a real route on a throwaway
Valhalla); if it fails nothing has changed anywhere and the basemap is not published. Between
steps 3 and 4 the map can lag the routing by a month's worth of new streets, which nothing depends
on.

What each step costs the running system:

| Step | Box | Users |
|---|---|---|
| build | nothing | nothing |
| rsync | ~360 MB in, capped by `--bwlimit` (KB/s) | nothing |
| switch | a second Valhalla for the check (~1 GB peak), then a restart | routing down a few seconds (`/geo/eta` falls back to a straight line); address search keeps the old places until the import's one transaction commits |
| publish | nothing | maps loading their style from then on get the new build; open maps keep reading the previous one until they reload (it stays on R2) |

`publish.sh` uploads `<date>/` (one multipart upload for the basemap, ~800 small fonts and
sprites), then the two styles, last, then deletes every dated folder except the new one and the one
that was live. The styles are regenerated for `https://tiles.taypi24.com/<date>`, never taken from
the build. Clients refetch the style within 5 minutes; tiles are never purged, the new date's URLs
are simply new.

## Rollback

Both halves, in the reverse order:

```bash
docker/geo/publish.sh 2026-09-01                  # PC: styles back to the previous date, nothing re-uploaded
GEO_DATA_DIR=/srv/geo docker/geo/rollback.sh      # box: current ⇄ previous, places re-imported
```

`publish.sh` finds a build under `<date>/`, or under `current/`/`previous/` after a local
`make geo-switch` renamed it (by its `BUILD` line). R2 and the box both keep exactly two builds, so
the previous date is always there on both. Running either rollback twice returns to where it started.

## Basemap size

Cloudflare's free plan caches files up to 512 MiB. Bolivia's basemap is 503 MiB, and `publish.sh`
refuses anything over `TILES_MAX_BYTES` before uploading. If an update crosses it, rebuild with
`--maxzoom=14` added to the `java -jar` line of `build.sh`: MapLibre overzooms 14, the street
detail stays. A file over Cloudflare's real limit would still be served, uncached, from R2 — the
second `curl` of the cut-over checks shows `Cf-Cache-Status: HIT` when it is cached.

## Basemap on R2 — one-time setup

1. Cloudflare → R2 → create bucket `tiles`.
2. R2 → Manage API tokens → Object Read & Write, **this bucket only**. Its endpoint and keys are
   the PC's `TILES_S3_ENDPOINT`, `TILES_S3_ACCESS_KEY`, `TILES_S3_SECRET_KEY`; the box never has them.
3. Bucket → Settings → CORS policy (the same as `docker/caddy/tiles.caddy`):

   ```json
   [{ "AllowedOrigins": ["*"], "AllowedMethods": ["GET", "HEAD"], "AllowedHeaders": ["Range", "If-Match"],
      "ExposeHeaders": ["ETag", "Content-Range", "Content-Length"], "MaxAgeSeconds": 86400 }]
   ```

4. `docker/geo/publish.sh <live date>` — the bucket is ready before any DNS change.
5. Cut-over, off-hours: delete the `tiles` A record, then Bucket → Settings → Custom Domains →
   connect `tiles.taypi24.com` (it creates the proxied record and the certificate).
6. Caching → Cache Rules → hostname equals `tiles.taypi24.com` → **Eligible for cache**, Edge TTL
   **use cache-control header if present**, Browser TTL **respect origin TTL**. Without the rule
   `.json`, `.pbf` and `.pmtiles` are not cached at all; without the Browser TTL the zone's default
   (4 h) replaces the styles' 5 min and a publish takes hours to reach browsers.
7. Maps pick up the new host as they reload their style (≤5 min). Then deploy the compose without
   the tiles site and drop `/srv/geo/*/tiles` from the box.

Checks, from any machine but the box:

```bash
curl -s https://tiles.taypi24.com/style-light.json | grep -o '[0-9-]*/basemap.pmtiles'   # the live date
curl -sI -H 'Range: bytes=0-16383' https://tiles.taypi24.com/<date>/basemap.pmtiles \
  | grep -iE '^HTTP|cf-cache-status|access-control-allow-origin'   # 206, *; run twice → HIT
```

Free-tier headroom: two builds are ~1 GB of the 10 GB; a publish is ~800 writes of the 1 M/month;
Cloudflare reads the basemap from R2 once per edge and serves ranges from its cache.

## Variables

| Variable | Where | Default | Meaning |
|---|---|---|---|
| `GEO_REGIONS` | build | `south-america/bolivia` | Geofabrik paths, comma-separated; several are merged into one extract |
| `TILES_URL` | build, publish | build: `http://127.0.0.1:8090/tiles` · publish: `https://tiles.taypi24.com` | build: baked into the build's (dev) styles · publish: the bucket's custom domain |
| `GEO_STYLE_LANG` | build, publish | `es` | label language; falls back to the local `name` |
| `GEO_BUILD_MEMORY` | build | `3g` | JVM heap for the basemap |
| `TILES_S3_ENDPOINT` | publish | — | `https://<account id>.r2.cloudflarestorage.com` |
| `TILES_S3_BUCKET` | publish | `tiles` | |
| `TILES_S3_ACCESS_KEY`, `TILES_S3_SECRET_KEY` | publish | — | the bucket-scoped R2 token |
| `TILES_MAX_BYTES` | publish | `536870912` (512 MiB) | the size guard |
| `GEO_DATA_DIR` | all | `docker/geo/data` | the layout above; compose mounts it too |
| `GEO_COMPOSE` | switch, rollback | `docker-compose.prod.yml` | the compose file that runs `valhalla` |
| `GEO_IMPORT` | switch, rollback | `docker compose … run migrate ./cli geo import …` | the command that loads `current/places.geojsonl` |
| `GEO_CHECK_ROUTE` | switch | `-17.3935,-66.1570;-17.3700,-66.1450` | two points inside the region, `lat,lng;lat,lng` |

The API's own `GEO_VALHALLA_URL`, `GEO_TIMEOUT`, `GEO_FALLBACK_SPEED_KMH` are in `.env.example`.

## Sizing

Bolivia (174 MB extract): 273 MB of routing tiles, a 503 MiB basemap, 125 k places. `valhalla`
runs 2 threads under `mem_limit: 1g` and sits near 60 MB idle. On a 4 GB box add 2 GB of swap
for the peaks of Valhalla's memory-mapped tiles alongside PostgreSQL:

```bash
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

## Locally

From `api/`: `make geo-build`, `make geo-switch DATE=<date>`, `make geo-up` (valhalla on :8002,
tiles on :8090, behind the compose `geo` profile so `make docker-up` works without a build). The
dev switch imports into `.env.platform`'s database and serves the build's own `tiles/`. Use
`127.0.0.1`, never `localhost`: Docker Desktop's IPv6 forward can accept a connection and never
answer.

## Licence

The data is © OpenStreetMap contributors, under the ODbL. Every map that shows it credits it:
`MapView` in the design system always shows the credit, uncollapsed; the mobile apps use the same
style URL and must do the same.
