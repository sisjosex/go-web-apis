# Geo stack — routing, basemap, address search (INFRA-003)

One OpenStreetMap extract feeds three things, all served from our own boxes:

| Piece | Built into | Served by |
|---|---|---|
| Routing tiles | `valhalla/tiles.tar` | the `valhalla` service (`/geo/route`, `/geo/eta` in the API) |
| Basemap, styles, fonts, sprites | `tiles/` | Caddy, at the root of `https://$TILES_HOST/` (dev: `http://127.0.0.1:8090/tiles/`) |
| Named streets, places, POIs | `places.geojsonl` | `geo.places` in the platform database (`/geo/geocode`, `/geo/reverse`) |

The server never builds: a build peaks at 2-4 GB of RAM and downloads a few GB. It runs on any
machine with Docker, the result is uploaded, and the server only switches to it.

## Layout of `$GEO_DATA_DIR`

```
sources/        download cache: Geofabrik extracts, water/land polygons, Natural Earth,
                landcover, fonts (~2.5 GB, reused by every build)
2026-10-01/     a finished build, not live yet
current/        what valhalla, Caddy and geo.places serve
previous/       the build before it — rollback.sh swaps the two
```

`current/BUILD` says which build is live: date, regions, pinned refs, the `TILES_URL` baked in.

## Monthly refresh

On the build machine, from `api/`:

```bash
GEO_REGIONS=south-america/bolivia TILES_URL=https://tiles.taypi24.com \
  docker/geo/build.sh 2026-10-01          # ~15 min warm, longer the first time
rsync -a docker/geo/data/2026-10-01 server:/srv/geo/
```

On the server, from the deploy checkout (`GEO_DATA_DIR=/srv/geo` in `.env`):

```bash
GEO_DATA_DIR=/srv/geo docker/geo/switch.sh 2026-10-01
```

`switch.sh` serves the new build from a throwaway Valhalla and routes across the region first
(`GEO_CHECK_ROUTE`, default central Cochabamba) — a bad build fails there and nothing changes.
Then `previous/` is deleted, `current/` becomes `previous/`, the build becomes `current/`,
`valhalla` is recreated on it, and `geo.places` is replaced in one transaction. Routing is down for
the few seconds valhalla takes to restart; `/geo/eta` answers from its fallback meanwhile.

Something wrong after the switch:

```bash
GEO_DATA_DIR=/srv/geo docker/geo/rollback.sh     # current ⇄ previous, places re-imported
```

Running it twice returns to where it started.

## Variables

| Variable | Where | Default | Meaning |
|---|---|---|---|
| `GEO_REGIONS` | build | `south-america/bolivia` | Geofabrik paths, comma-separated; several are merged into one extract |
| `TILES_URL` | build | `http://127.0.0.1:8090/tiles` | public URL of `tiles/`, baked into both styles |
| `GEO_STYLE_LANG` | build | `es` | label language; falls back to the local `name` |
| `GEO_BUILD_MEMORY` | build | `3g` | JVM heap for the basemap |
| `GEO_DATA_DIR` | all | `docker/geo/data` | the layout above; compose mounts it too |
| `GEO_COMPOSE` | switch, rollback | `docker-compose.prod.yml` | the compose file that runs `valhalla` |
| `GEO_IMPORT` | switch, rollback | `docker compose … run migrate ./cli geo import …` | the command that loads `current/places.geojsonl` |
| `GEO_CHECK_ROUTE` | switch | `-17.3935,-66.1570;-17.3700,-66.1450` | two points inside the region, `lat,lng;lat,lng` |

The API's own `GEO_VALHALLA_URL`, `GEO_TIMEOUT`, `GEO_FALLBACK_SPEED_KMH` are in `.env.example`.

## Sizing

Bolivia (174 MB extract): 273 MB of routing tiles, a 515 MB basemap, 125 k places. `valhalla`
runs 2 threads under `mem_limit: 1g` and sits near 60 MB idle. On a 4 GB box add 2 GB of swap
for the peaks of Valhalla's memory-mapped tiles alongside PostgreSQL:

```bash
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

## Locally

From `api/`: `make geo-build`, `make geo-switch DATE=<date>`, `make geo-up` (valhalla on :8002,
tiles on :8090, behind the compose `geo` profile so `make docker-up` works without a build). The
dev switch imports into `.env.platform`'s database. Use
`127.0.0.1`, never `localhost`: Docker Desktop's IPv6 forward can accept a connection and never
answer.

## Licence

The data is © OpenStreetMap contributors, under the ODbL. Every map that shows it credits it:
`MapView` in the design system always shows the credit, uncollapsed; the mobile apps use the same
style URL and must do the same.
