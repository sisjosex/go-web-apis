# AGENTS.md — Xanthops API

Orientation for agents working in `/api`. Coding rules load on their own by file type from
`.claude/rules/` (`go.md`, `sql.md`, `go-tests.md`, `security.md`) — this file is only what you need
before you open a file: where things are, how they run, and the gotchas that are not visible in code.

## Non-negotiable

- Business logic lives in PostgreSQL SPs. Go binds, calls, maps error codes, responds.
- `tenant_id` only from `c.Get("tenant_id")`; tenant routes always carry auth + tenant middleware.
- Secrets only from `.env.*`.
- A change ships with `make gate MODULES="<modules touched>"` green. `make build` alone is not
  verification. New or changed endpoint ⇒ integration test in `modules/{name}/tests/`.
- Changing an SP signature ⇒ `DROP FUNCTION IF EXISTS` + `CREATE FUNCTION`, never `CREATE OR REPLACE`.

## Commands — from `api/`

```bash
make dev-platform | dev-tenant        # hot reload, ports 8080 / 9081
make build                            # compile to bin/
make gate MODULES="tracking"          # check + lint (changed code) + build + tests of MODULES, quiet
make check-all | lint-all             # the whole-tree baseline, when asked for
make docker-up | docker-down          # PostgreSQL + Valkey containers (tests need them up)
make test-<module>                    # auth core users tenancy tracking inventory sales purchasing billing geo
make test-all                         # every module (db-reset first)
make migrate MODULE=x NAME=y          # .up.sql + .down.sql pair
make tenant-migrate SLUG=x            # apply to one tenant DB
make swagger                          # regenerate docs
make geo-build | geo-switch DATE=… | geo-rollback | geo-up   # geo data, docker/geo/README.md
```

Setup is human-only: copy `.env.example` → `.env.platform` / `.env.tenant`, then `make docker-up`.

## Shape

**Stack** Go 1.22+ · Gin · pgx/v5 · PostgreSQL · SQL migrations, no ORM.

| Server | Entry | Env | Port | Modules |
|---|---|---|---|---|
| Platform | `cmd/server -mode=platform` | `.env.platform` | 8080 | core auth users tenancy billing |
| Tenant | `cmd/server -mode=tenant` | `.env.tenant` | 9080 | core auth users tracking inventory sales purchasing geo |

One binary, one `-mode` flag: `cmd/platform` and `cmd/tenant` have not existed for some time. The
sibling `cmd/cli` carries `migration` (generate), `migrate` (apply, one shot), `tenant`, `jobs` and
`geo import`.

**Roles (INFRA-001).** `-role` (flag > `APP_ROLE` > `all`) picks what a process runs: `api`, `worker`
(asynq handlers + outbox relay + GPS stream consumer), `scheduler` (periodic entries, fired only while it holds the
`jobs:scheduler:lease` in Valkey), `realtime` (tenant, the WebSocket gateway alone: `GET /tracking/ws`; `all` runs it too). Worker and scheduler exist
in platform mode only — they walk the tenants `tenancy.*` lists. Non-API roles serve only the probes.
Wiring is `cmd/server/background.go`: a module adds a handler or a periodic entry in `registerJobs`,
never in `core/jobs`. `REDIS_URL` unset ⇒ `all` is the API alone; Valkey down ⇒ the rate limit fails open
and `/readyz` answers 200 `degraded` (503 for worker/scheduler). asynqmon is at `/admin/jobs`, super_admin.

**Outbox.** A side effect that must survive a crash is a row in `tracking.outbox`, inserted by the SP that
makes the change, in its transaction. The relay publishes it as asynq task `outbox:<topic>` (TaskID
`<tenant>:<id>`); handle the topic in `registerJobs`. Delivery is at least once — handlers are idempotent.

**Trips (TRACK-008).** `tracking.sp_materialise_trips` is the only writer of planned trips: the
`outbox:route.changed` handler calls it for one route (tenant taken from the TaskID), the daily
`tracking:trips-materialise` entry (02:00 UTC) for every route. Both are clamped to each route's local
today..today+14 (`routes.timezone`, IANA). An SP that changes a route's plan or riders writes
`route.changed` via `tracking.fn_route_changed` and never touches trips itself.

Trips move over HTTP (TRACK-020): `POST /trips/:id/start|complete|cancel`, `/trip-stops/:id/arrive|skip`,
`/trip-stop-tasks/:id/done|no-show` (body `client_op_id`, idempotent), `PATCH /trips/:id` (override →
`is_overridden`). Each SP locks the trip row, writes `trip_events` + a `trip.changed` outbox row through
`tracking.fn_trip_changed` and answers the detail; the `outbox:trip.changed` handler stores a completed
trip's driven path (`polyline`, `distance_km`, `trace_source`: Valhalla `trace_attributes`, the raw GPS
line when it is down), which only `GET /trips/:id` answers.

**GPS (TRACK-010).** `POST /tracking/ingest/positions` (≤ 500 points, 202 `{accepted, path}`) takes two
ways in, chosen by header: `X-Device-Token: <tenant_id>.<secret>` (`tenancy.gps_devices`, sha256 only,
issued by `cli tenant -gps-device <slug> -vehicle <uuid>`) or a driver's Bearer + slug (vehicle of their
trip in progress, else 409). Points go to the Valkey stream `gps:{tenant}` (Valkey down → stored inline,
`path: stored`); the worker's `PositionsConsumer` reads 1 000 at a time into `sp_ingest_positions`, the
only writer of `vehicle_positions` (UTC-day partitions, `tracking:positions-partitions` 01:00 UTC keeps a
week ahead and drops past `TRACKING_LOCATION_RETENTION_DAYS`) and `vehicle_last_position` (what every
"where is it now" read looks up), marks the next stop arrived within `GPS_ARRIVAL_RADIUS_M`, then
publishes `{type, data}` to `fleet:{tenant}`, `fleet:{tenant}:org:{id}`, `trip:{id}`, `rider:{id}` —
a position at most every 3 s per vehicle, an arrival at once. Load: `go run ./tools/gpsload`.

**Driver's phone (TRACK-011).** `/api/v1/mobile/driver/*`, the ingest's driver chain + `RequireTenantRole(driver)`,
no capability; each SP resolves the driver from `drivers.user_id` (none → 403 `driver.not-linked`, another
driver's trip → `trip.not-found`). `GET /today` = `sp_driver_today`, `ETag` = sha256 of the SP's JSONB
(`generated_at` added after, outside the hash) → `If-None-Match` 304. `POST /trips/:id/start`. `POST /sync
{ops ≤ 100}` = `sp_driver_sync`: ops by `client_recorded_at`, each in its own subtransaction, logged in
`tracking.driver_ops` (PK `client_op_id`) — a replay answers the stored `replayed`/`rejected + code`
without running. The transition SPs take `p_recorded_at` (clamped to [started_at, now]; NULL = now) for
`arrived_at`/`done_at`/`ended_at`.

**Rider card and incidents (TRACK-027).** `riders.qr_token` (32 base32 chars, `fn_new_qr_token()`, UNIQUE across
the table — riders have no `tenant_id`): `GET|POST /tracking/riders/:id/qr-token` (POST rotates, riders:write).
`GET /mobile/driver/riders/:code` (case and blanks ignored; unknown/foreign → 404 `rider.not-found`) and `GET
/mobile/driver/riders?q=` (name or code, only riders on the driver's trips in progress) answer `{rider, task}`,
`task` = the pending pickup on the trip in progress or null. `POST /mobile/driver/incidents` = `sp_driver_incident`:
a `route_alerts` row with `trip_id` + `location`, plus an `incident` trip event and `trip.changed` via `fn_trip_changed`.

**Realtime (TRACK-025).** `GET /tracking/ws` (`modules/tracking/realtime`): a browser offers the JWT as the
subprotocol pair `bearer, <jwt>` and the tenant as `?tenant_slug=`; then `{op: subscribe|unsubscribe,
channel}` ↔ `{channel, type, seq, data}` (`type` position|stop|trip_status|task|eta, or `error` with
`code: forbidden`, the socket kept). Each subscribe is `sp_can_subscribe`, cached 5 min per (user,
channel): operators get `fleet:{tenant}[:org:{id}]`, `trip:{id}`, `rider:{id}`; a guardian only its own
`rider:{id}` while it rides; organization and driver levels are refused at the upgrade. One Valkey
SUBSCRIBE per channel per process; position and eta are latest-wins per channel, anything else queues
and past `WS_QUEUE_MAX` closes 1013; Valkey lost or shutdown closes 1012. `outbox:trip.changed` publishes
every transition; the hub adds `eta` on a watched `trip:{id}`. ETA is on demand (D1): one Valhalla route
through every pending stop, cached `eta:{trip}` 30 s. Snapshots for (re)sync: `GET /tracking/live/fleet`,
`GET /tracking/trips/:id/live`.

Main DB holds `auth.*` + `tenancy.*`; one DB per tenant holds business schemas only. JWT carries
`user_id + session_id`; tenancy middleware resolves `tenant_id` from `X-Tenant-Slug` per request.

Middleware order: CORS → rate limit → language (`lang`) → auth (`user_id`) → tenant (`tenant_id`) →
optional `require-module-access`.

```
modules/{name}/
├── controllers/  services/  repositories/  interfaces/  models/
├── routes/       # RegisterXRoutes, gated by IsModuleEnabled in routes/routes.go
├── config/       # LoadXConfig → config.ModularAppConfig.X ; env via utils.GetEnv*
├── migrations/   # {timestamp}_{name}.up.sql / .down.sql
├── lang/         # en.json, es.json — flat keys "module.action.error-type"
├── errors/       # error code constants
└── tests/        # helpers.go + {name}_api_test.go, build tag integration
```

Reference files: `routes/routes.go` (DI wiring, middleware order) · `config/config.go` ·
`modules/core/errors/error.go` (`BuildError*`) · `modules/auth/controllers/auth_controller.go`
(canonical controller) · `modules/inventory/tests/` (canonical tests).

**Geo (INFRA-003).** `modules/geo`, tenant server only: `GET /geo/geocode`, `/geo/reverse`,
`POST /geo/route`, `/geo/eta`, behind auth + tenant. `geo.places` is OSM, the same for every tenant,
so it lives in the server's own database and the repository always takes the primary pool; tenant
databases never get the schema (`tenant_service.go` excludes it). Routing goes through the
`interfaces.Router` port (Valhalla adapter, gobreaker: 5 failures → open 30 s); a 4xx from Valhalla is
`ErrNoRoute` (422) and never opens the breaker. `/geo/eta` always answers: Valhalla → observed
durations (nil until TRACK-010) → straight line × 1.3, flagged in `estimate_source`.

## REST conventions

`/api/v1/{resource}` plural kebab-case · `Authorization: Bearer` · `X-Tenant-Slug` · errors via
`BuildErrorSingle` (one code) / `BuildErrorDetail` (binding, field detail) / `BuildError` (unexpected).

**List endpoints have no uniform contract — read the handler before writing a client or a test.**
Verified 2026-09-19 (APP-004):

| Endpoint | Params | Body |
|---|---|---|
| `GET /users`, `/users/audit` | `page limit search` + `status sort order` / `action from to` | `{ users \| entries, total, page, limit, total_pages }` |
| `GET /inventory/products` | `page page_size search category_id` (cap 100) | `{ products, total_count, page, page_size }` |
| `GET /inventory/batches/product/:id` | `page page_size onlyActive skuId` (cap 100) | `{ batches, total_count, page, page_size }` |
| `GET /inventory/batches/expiring` | `page page_size warningDays` (cap 100) | `{ batches, total_count, page, page_size }` |
| `GET /inventory/movements` | `page page_size product_id sku_id movement_type date_from date_to` | `{ movements, total_count, page, page_size }` |
| `GET /inventory/categories` | `page limit` | `{ categories, total }` |
| `GET /purchasing/suppliers`, `/orders` | `page page_size` (cap 100) | `{ <plural>, total_count, page, page_size }` |
| `GET /tracking/companies` | `page page_size search status` (cap 100) | `{ companies, total_count, page, page_size }` |
| `GET /tracking/vehicles` | `page page_size search company_id vehicle_type status` (cap 100) | `{ vehicles, total_count, page, page_size }` — every row carries `company_name` |
| `GET /tracking/drivers` | `page page_size search company_id status` (cap 100) | `{ drivers, total_count, page, page_size }` — every row carries `company_name` and `user_email` |
| `GET /tracking/routes` | `page page_size search company_id direction is_active` (cap 100) | `{ routes, total_count, page, page_size }` — search is name or code; every row carries `company_name`, `license_plate`, `driver_name`, `capacity` (the route's override, else its vehicle's) and `capacity_override` (PATCH `capacity: 0` clears it); `GET/POST/PATCH` of one route answer the same row |
| `GET /tracking/documents` | `page page_size subject_type subject_id expiring_within_days` (cap 100) | `{ documents, total_count, page, page_size }` — every row carries `type_name`, `subject_name` and a `status` of valid/expiring/expired |
| `GET /tracking/document-types` | `applies_to is_active` | array — the tenant's whole policy, unpaginated |
| `GET /tracking/organizations` | `page page_size search kind is_active` (cap 100) | `{ organizations, total_count, page, page_size }` |
| `GET /tracking/stop-places` | `page page_size search near radius_m` (cap 100) | `{ stop_places, total_count, page, page_size }` — `near=lat,lng` narrows to `radius_m` metres (default 1000), adds `distance_m` to every row and orders nearest first |
| `GET /tracking/routes/:id/stops` | `date` (YYYY-MM-DD, default today) | array — the stops of the route version in force that day; every row carries `version_id` and `stop_place_id` |
| `GET /tracking/routes/:id/versions` | — | array, newest first — each version with `effective_from`, `effective_to` and `stops_count` |
| `GET /tracking/calendars` | `page page_size search` (cap 100) | `{ calendars, total_count, page, page_size }` — every row carries `dates_count` |
| `GET /tracking/calendars/:id/dates` | `from to` (YYYY-MM-DD, both optional) | array, oldest first — `date`, `kind` of no_service/special_service, `label`; `PUT` replaces the whole list with a bare array of the same shape |
| `GET /tracking/routes/:id/schedules` | — | array, newest validity first — `days_of_week` is a bitmask (bit 0 Monday), `start_time` is HH:MM, every row carries `calendar_name` |
| `GET /tracking/routes/:id/exceptions` | `date_from date_to` (overlap, not containment) | array, oldest first — `kind` and a `payload` whose keys the kind defines |
| `GET /tracking/routes/:id/preview` | `from to` (YYYY-MM-DD, both required, at most 92 days apart) | array — one row per departure the plan produces, with `service_date`, `schedule_id`, `start_time`, `version_id`, `vehicle_id`, `driver_id`, a `status` of planned/cancelled and the `exceptions` that apply; a day the route does not run has no row |
| `GET /tracking/assignments` | `page page_size rider_id route_id date` (cap 100) | `{ assignments, total_count, page, page_size }` — `date` narrows to the assignments in force that day; every row carries `rider_name`, `route_name`, `direction`, `pickup_stop_name`, `dropoff_stop_name`; `days_of_week` is the schedules' bitmask. Writes (`POST`, `PATCH /:id`, `POST /bulk`) answer `{ assignment \| assignments, warnings }` |
| `GET /tracking/trips` | `page page_size date route_id status organization_id` (cap 100) | `{ trips, total_count, page, page_size }` — `date` defaults to each route's local today; every row carries `route_name`, `direction`, `license_plate`, `driver_name`, `stops_count`, `tasks_total` (not cancelled), `tasks_done` (done or no_show), `delay_seconds` (list only: next pending stop overdue by, 0 not due, null once ended); `GET /:id` and every trip write answer the row plus `stops[].tasks[]`; `GET /:id` also `polyline`, `distance_km`, `trace_source` (null until the trip completes) |
| `GET /tracking/live/fleet` | `organization_id` | `{ vehicles: [{ vehicle_id, license_plate, trip_id, route_name, lat, lng, speed, heading, recorded_at }] }` — every vehicle that has reported, unpaginated; the org filter keeps vehicles whose trip carries its riders |
| `GET /tracking/trips/:id/live` | — | the `GET /:id` detail plus `position` (the last position frame, or null) and `eta: [{ trip_stop_id, eta_at, estimate_source }]` per pending stop, empty unless the trip is in progress and has a position |
| `GET /tracking/riders` | `page page_size search organization_id rider_type is_active` (cap 100) | `{ riders, total_count, page, page_size }` — every row carries `organization_name`; every rider shape carries `home_latitude`, `home_longitude`, `notes` |
| `GET /tracking/riders/:id/absences` | `from to` (overlap, both optional) | array, oldest first — `direction` null is both, `reported_via` web/portal, `reported_by_name`; on the portal chain (a guardian sees own riders). `POST` answers `{ absence, cancelled_tasks }`; 409 `overlap`, `cutoff` (guardian inside the organization's `absence_cutoff_min`) |
| `GET /tracking/riders/:id/suggestions` | `direction` (required) `days` (bitmask, 127) `max_walk_m` (800, ≤ 3000) `limit` (10, ≤ 50) | `{ suggestions }` — straight line from `home_location`, nearest first then most `free_seats`; 409 `tracking.rider.no-home-location` |
| `GET /sales/orders` | `limit offset` | `{ data }` — no total |
| `GET /sales/customers`, the other `/tracking/*`, `/inventory/stock/:id` | filters only | array, object or `{ data }`, unpaginated |

New list endpoints use the purchasing shape. No total ⇒ the app can only "load more"; no `search`
param ⇒ search is client-side or new API work — the spec decides, never the implementation.

## Adding a module

1. Layout above · 2. `config/config.go` + register in `ModularAppConfig` · 3. migrations · 4. `lang/` ·
5. `errors/` · 6. interfaces → repository → service → controller · 7. routes in `routes/routes.go` gated
by `IsModuleEnabled` · 8. add to `ENABLED_MODULES` in the env file **and `.env.test`, after every module
whose schema it references** — that list is the migration order · 9. `tests/helpers.go` + API tests ·
10. `test-{name}` Makefile target.

## Deploy

`docker-compose.prod.yml` — one image (`Dockerfile` builds `app` + `cli`), one PostgreSQL 17 with
PostGIS on a named `pgdata` volume, and Caddy as the only container publishing a host port (80/443,
automatic TLS). Neither the database nor either API is reachable from outside.

Servers run with `SKIP_MIGRATIONS=true`: migrations belong to the one-shot `migrate` /
`migrate-tenant` jobs, which the APIs wait on (`service_completed_successfully`). Locally the servers
still migrate on start — one replica, nothing to race with. Local development runs on the **native
PostgreSQL 16 service on 5432**, which needs the PostGIS bundle installed (StackBuilder); the compose
`platform-db` publishes 5434 and `tenant-db` 5433 (`.env.test`). A migration that fails leaves its
module dirty and the server keeps retrying with the error in the log — it is never forced past: run
the file's `.down.sql`, fix the cause, reset `schema_migrations_<module>`. `/livez` (always 200) and `/readyz` (200
when the pool pings, else 503; Valkey per role, see Roles) back the healthchecks; both are registered before the CORS and rate
limit middleware, so a probe is never rate-limited.

Nothing in the API issues `CREATE DATABASE` — the tenancy service only migrates a URL it is handed.
The tenant server's own database is created by `docker/postgres/init-extra-databases.sh` from
`POSTGRES_EXTRA_DBS`, and a per-tenant database is created by hand before `cli tenant -migrate <slug>`.
The users module runs against tenant databases too, which carry `auth.*` but no `tenancy.*`: every
`tenancy` reference in a `users` migration must be guarded by `to_regclass`.

Valkey (AOF `everysec`, `valkeydata` volume, `VALKEY_PASSWORD`) backs `api-worker` and `api-scheduler`,
the same image in `-role=worker|scheduler`; the two APIs run `-role=api`. A second scheduler is safe
(lease), a second worker scales throughput.

`valhalla` serves routing from `${GEO_DATA_DIR}/current`, Caddy the basemap under `TENANT_HOST/tiles/`;
builds are made off the server and put live by `docker/geo/switch.sh` (`docker/geo/README.md`).

`docker/backup/README.md` has the nightly dump and the restore steps; deploy variables are at the
bottom of `.env.example`. PITR, PgBouncer and monitoring are INFRA-004.

## Gotchas not visible in code

- `make test-*` runs `db-reset` first: drops, recreates, migrates and seeds `josex_test`. A `db-reset`
  failure is a real current problem, not a known issue to step around.
- Adding a column to an SP without adding it to `row.Scan` fails silently at runtime, not at build.
- A test file without `//go:build integration` is excluded and the module looks green.
- `t.Skipf` on an unexpected status hid a 500-ing sales flow for months. Use `t.Fatalf`.
- Module error codes are matched by exact string in the app — renaming one is an app change too.
- Local `REDIS_URL` uses `127.0.0.1`: Docker Desktop's IPv6 forward for `localhost` can accept a
  connection and never answer, so go-redis times out while `valkey-cli` inside the container works.
- Docker Desktop cannot run Valhalla's tile build on a Windows bind mount (its memory-mapped scratch
  files segfault): `docker/geo/build.sh` builds on the container's disk and copies results out.
- In `modules/tracking/tests/` and `modules/geo/tests/` a test file must sort after `helpers.go`
  (`helpers.go` declares the `_test` package; an earlier `_test.go` file is read as an external test and
  the package fails to load).
