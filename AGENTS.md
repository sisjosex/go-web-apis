# AGENTS.md — Taypi API

Orientation for agents working in `/api`. Coding rules load on their own by file type from
`.claude/rules/` (`go.md`, `sql.md`, `go-tests.md`, `security.md`) — this file is only what you need
before you open a file: where things are, how they run, and the gotchas that are not visible in code.

## Non-negotiable

- Business logic lives in PostgreSQL SPs. Go binds, calls, maps error codes, responds.
- `tenant_id` only from `c.Get("tenant_id")`; tenant routes always carry auth + tenant middleware.
- Secrets only from `.env.*`.
- A change ships with `make gate` green. `make build` alone is not verification. The gate picks the
  modules to test from `git diff` — every module when `core`, `auth`, `tenancy`, `users`, `routes/`,
  `cmd/` or `config/` changed; `MODULES="x y"` overrides it.
- Every route has a test that reaches it with a **2xx**, and no test request hits a path no route
  answers. The gate enforces both after each module's tests (INFRA-014). New or changed endpoint ⇒
  integration test in `modules/{name}/tests/` that drives it to success, not only its refusals.
- A function lives in `modules/{name}/sql/functions/{fn}.sql` (`CREATE OR REPLACE`, edited in place), applied
  after the module's migrations whenever its sha256 changes; a migration only changes tables and never
  creates a function. A committed migration is never edited. Rules in `sql.md` (INFRA-015).

### Reading the route check

`make test-<m>` records every request into `.route-hits`; the gate then runs
`go run ./tools/check -routes .route-hits -module <m>`. A route belongs to the module of its handler's
package. Two kinds of failure:

- `route without a 2xx test hit: PATCH /api/v1/x/:id — never reached` — no test calls it; `— reached
  with 400, 404` — tests call it but none succeeds. Add the success case.
- `unmatched test request (no route answers it — wrong method or path): PATCH /api/v1/x/abc (404)` — a
  test asserts on a route that does not exist (wrong verb, a typo, a removed route). Fix or delete it.

Routes whose handler lives in `routes/` (the jobs monitor UI, `/tracking/ws`, `/openapi.json`, `/docs`) are not checked.

## Commands — from `api/`

```bash
make dev-platform | dev-tenant        # hot reload, ports 8080 / 9081
make build                            # compile to bin/
make gate                             # check + lint (changed code) + build + tests and route coverage of the changed modules, quiet
make check-all | lint-all             # the whole-tree baseline, when asked for
make docker-up | docker-down          # PostgreSQL + Valkey + local S3 (:8333) containers (tests need them up)
make seed-dev                         # emulator dev accounts on tenant mi-negocio (cmd/cli/seed_dev.sql), idempotent
make test-<module>                    # auth core users tenancy tracking inventory sales purchasing billing geo
make test-all                         # every module (db-reset first)
make migrate MODULE=x NAME=y          # .up.sql + .down.sql pair — tables only
make schema                           # db-reset, then rewrite every sql/schema.sql (commit it with the migration)
make tenant-migrate SLUG=x            # apply to one tenant DB
make contract                         # write docs/openapi.json + docs/routes.json from the routes' types (commit both)
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
sibling `cmd/cli` carries `migration` (generate), `migrate` (apply, one shot), `functions dump` and `schema dump`
(write `sql/` from a reset database), `tenant`, `jobs`,
`geo import` and `tracking paths backfill`.

**Roles (INFRA-001).** `-role` (flag > `APP_ROLE` > `all`) picks what a process runs: `api`, `worker`
(asynq handlers + outbox relay + GPS stream consumer), `scheduler` (periodic entries, fired only while it holds the
`jobs:scheduler:lease` in Valkey), `realtime` (tenant, the WebSocket gateway alone: `GET /tracking/ws`; `all` runs it too). Worker and scheduler exist
in platform mode only — they walk the tenants `tenancy.*` lists. Non-API roles serve only the probes.
Wiring is `cmd/server/background.go`: a module adds a handler or a periodic entry in `registerJobs`,
never in `core/jobs`. `REDIS_URL` unset ⇒ `all` is the API alone; Valkey down ⇒ the rate limit fails open
and `/readyz` answers 200 `degraded` (503 for worker/scheduler). asynqmon is at `/admin/jobs`, super_admin.

**Outbox.** A side effect that must survive a crash is a row in `tracking.outbox`, inserted by the SP that
makes the change, in its transaction. Each row names its tenant: a BEFORE INSERT trigger fills `tenant_id`
from `payload.tenant_id` or `payload.route_id`, and refuses a row with neither (`outbox.tenant`) — a new
topic carries one of the two. The relay runs one `LISTEN` per **database** (the shared platform one and
each dedicated one, INFRA-009) and publishes each row as asynq task `outbox:<topic>` (TaskID
`<row tenant_id>:<id>`); handle the topic in `registerJobs` and resolve the tenant with `taskTenant`, which
reads the in-memory `jobs.TenantDirectory`. Delivery is at least once — handlers are idempotent.

**Trips (TRACK-008).** `tracking.sp_materialise_trips` is the only writer of planned trips: the
`outbox:route.changed` handler calls it for one route (tenant taken from the TaskID), the daily
`tracking:trips-materialise` entry (02:00 UTC) for every route. Both are clamped to each route's local
today..today+14 (`routes.timezone`, IANA). An SP that changes a route's plan or riders writes
`route.changed` via `tracking.fn_route_changed` and never touches trips itself. The one exception is
`tracking:trips-cancel-empty` (every 5 min, TRACK-069): `sp_cancel_empty_trips` cancels today's trips still
without riders once their client's `empty_trip_cancel_min` is reached (overridden, so never revived; event
reason `no_riders`, no guardian notice) and the job pushes the driver `trip_cancelled_empty`. A rider added
to a running trip by the materialiser is a `rider_added` event and notice.

**Planned route line (TRACK-028).** `route_versions` keeps the line by streets (`planned_polyline`
precision 6, `planned_distance_m`, `planned_legs [{distance_m, duration_s}]`, `path_source`
valhalla|fallback, `path_points_hash`, `path_computed_at`). The `outbox:route.changed` handler, after the
trips, recomputes every version in force from `date_from` (`jobs.PathPlanner`): an unchanged points hash
costs no router call; router down or no route → straight line `fallback` and the task fails, so asynq
retries it. `GET /tracking/routes/:id/path?date=` (`sp_get_route_path`, scoped as the stops) →
`{path | null}`, `ETag` = version + `path_computed_at`, `If-None-Match` → 304 (CORS allows it and exposes
`ETag`). `cli tracking paths backfill` writes a `route.changed` row per route with an uncomputed version.
`/mobile/driver/today`'s trips carry the same line as `planned_path {polyline6, distance_m, legs, source} |
null`, read through `trips.route_version_id` in the SP's statement (MOBILE-015); a stored line moves its `ETag`.

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
`/today`'s riders carry `qr_token` (MOBILE-006) so the phone resolves a scan offline; a rotation changes the `ETag`.

**Realtime (TRACK-025).** `GET /tracking/ws` (`modules/tracking/realtime`): a browser offers the JWT as the
subprotocol pair `bearer, <jwt>` and the tenant as `?tenant_slug=`; then `{op: subscribe|unsubscribe,
channel}` ↔ `{channel, type, seq, data}` (`type` position|stop|trip_status|task|eta, or `error` with
`code: forbidden`, the socket kept). Each subscribe is `sp_can_subscribe`, cached 5 min per (user,
channel): operators get `fleet:{tenant}[:org:{id}]`, `trip:{id}`, `rider:{id}` and `nav:{tenant}` (the rail's `nav_counts`, pushed by alert/trip changes and the document pass, TRACK-081); a guardian only its own
`rider:{id}` while it rides; an organization user its trips' `trip:{id}` and its own `fleet:{tenant}:org:{id}`
(not a supervisor with groups, TRACK-067); a driver only `driver:{id}`. One Valkey
SUBSCRIBE per channel per process; position and eta are latest-wins per channel, anything else queues
and past `WS_QUEUE_MAX` closes 1013; Valkey lost or shutdown closes 1012. `outbox:trip.changed` publishes
every transition; the hub adds `eta` on a watched `trip:{id}`, and on a watched `rider:{id}` the same
trip ETA cut to that rider's stops (MOBILE-010). ETA is on demand (D1): one Valhalla route
through every pending stop, cached `eta:{trip}` 30 s; Valhalla down, each leg falls back to the
median of `stop_pair_samples` between the stops nearest its ends (≥ 3 samples in the weekday + 2-hour
band, else any band, 90 days; written at trip completion, pruned by the partitions walk), then the
straight line (TRACK-066). Every freshly computed ETA runs `sp_trip_delay_check`: a pending stop late
by its riders' clients' `delay_notice_min` (default 10, 0 never) is stamped `delay_notified_at` and
becomes one `trip.changed` `delay` → a `delay` notice with `delay_min`/`eta_at` to the families with a
pending task there, push text `notice.delay_eta.*`. Snapshots for (re)sync: `GET /tracking/live/fleet`,
`GET /tracking/trips/:id/live` (its stops carry `lat`/`lng`), and the guardian's `GET /tracking/riders/:id/live`
(portal chain, scoped as the rider's status; no trip in progress → `trip_id: null`, no stops, no position).
`GET /routes/:id/status` does not name the trip: a client finds it with `GET /trips?route_id=&status=in_progress`.
The gateway logs one line per socket open and close, at debug level.

**Reports (TRACK-013).** The reports read aggregates, never trips: `trip_summaries` (one row per ended
trip — km, minutes, pickups, no-shows, peak aboard, start/stop delays, alerts; `organization_ids` = its riders'
organizations), `rider_day_attendance` (each rider's pickup: boarded/no_show/absent/cancelled) and
`stop_delay_stats` (p50/p90 minutes late per route, stop and ISO weekday, 8 weeks). `sp_trip_summarise`
rolls a trip from scratch: the `outbox:trip.changed` handler calls it on complete/cancel, after the trace
so `distance_km` is in; the daily `tracking:report-rollups` (03:00 UTC) sweeps the last 3 days for trips
without a summary or changed after it, recomputes the stop stats, and writes yesterday's UTC usage
(`sp_usage_for_day`: vehicles that completed a trip or sent a point, trips, points, riders boarded) to the
platform's `tenancy.usage_daily`. `GET /tracking/reports/attendance|punctuality|fleet-usage|incidents`
(permission `tracking:reports:read`; an organization user reaches them, narrowed to its organizations) each
take `from to` (default the last 30 days, > 366 → 400 `range-too-wide`) and answer one SP row:
`{ rows, summary, total_count, page, page_size }`. `GET /tracking/reports/:report/export` writes the same
rows as CSV (UTF-8 BOM, headers `tracking.reports.csv.*` in the request's language); over
`services.ReportExportMax` (10 000) rows → 413 before any row. `GET /platform/usage?from&to&tenant_id`
(super_admin) reads `usage_daily` per tenant with `vehicles_active_max`/`_avg`; the billing rule over it is
not here.

Main DB holds `auth.*` + `tenancy.*`; one DB per tenant holds business schemas only. JWT carries
`user_id + session_id`; tenancy middleware resolves `tenant_id` from `X-Tenant-Slug` per request.

**Sessions (MOBILE-011).** The access token is not checked against the session per request: an ended
session goes at its next `POST /auth/refresh_token`, which validates it, stamps `last_active` and returns a
new `refresh_token` too (lifetime restarts, so a client in use never signs in again). `PUT /auth/password`
ends every other session, a reset ends all. `POST /auth/password/reset` answers `true` for any email and
never the token; `POST /auth/otp/verify` for an email without an active account is `otp.invalid`.

Middleware order: CORS → rate limit → language (`lang`) → auth (`user_id`) → tenant (`tenant_id`) →
optional `require-module-access`.

```
modules/{name}/
├── controllers/  services/  repositories/  interfaces/  models/
├── routes/       # RegisterXRoutes, gated by IsModuleEnabled in routes/routes.go
├── config/       # LoadXConfig → config.ModularAppConfig.X ; env via utils.GetEnv*
├── migrations/   # {timestamp}_{name}.up.sql / .down.sql — tables, indexes, views, data
├── sql/          # functions/{fn}.sql (one name each, its trigger too) · schema.sql (generated, read a table here)
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
durations → straight line × 1.3, flagged in `estimate_source`. The observed durations are tracking's
(TRACK-066): wired only into the trip ETA, which carries the tenant in the context; `/geo/eta` gets nil.
The tracking riders import takes the places repository as its `PlaceSearcher` (TRACK-072), built in
`registerImport` when geo is enabled, nil otherwise.

**Stored files (INFRA-007).** `core/services/storage`: the `ObjectStore` port (one bucket; one
minio-go client per process), R2 in prod, the compose's SeaweedFS in dev and tests (`.env.test` uses the
`-test` buckets; `storagetest.Media|Documents(t)` hands them to a test). No byte goes through the API:
`POST /storage/uploads {purpose, content_type, size}` signs a 5-min PUT to `pending/<tenant>/<uuid>.<ext>`,
type and length in the signature; the owning module's endpoint claims the key (range read 0-511 → sniff +
total size, copy to the final key, pending deleted after the answer, lifecycle sweeps the rest). A module
adds an upload by registering a `storage.Purpose` on `routeDeps.purposes` in `routes.go`. Private files go
out as 60 s `PresignGet` links. `MediaService` writes public images to the media bucket server-side (the
imports), type from the extension allow-list, `immutable` cache; the row stores the full URL.

## Handlers are huma operations (INFRA-021)

A JSON route is registered with `api.Get|Post|Put|Patch|Delete(group, "/x/{id}", ctrl.H, api.Doc{…}, mw...)`
from `modules/core/api`, and its handler is `func(ctx context.Context, in *HInput) (*api.Out[T], error)`.
The contract (`docs/openapi.json`) is built from those types, so the input struct and `T` *are* the docs.

- **Inputs.** Path params ``ID string `path:"id"` `` (the handler still parses the UUID, so a bad one answers
  what it always did); a raw query key ``Page int `query:"page" default:"1"` ``; a query DTO
  `Query api.Query[models.XQuery]` bound with `in.Query.Into(ctx, &q)`; a body `JSON api.Body[models.XDto]`
  bound with `in.JSON.Into(ctx, &dto)` — never a field named `Body` (huma would read the body itself).
  Binding stays Gin's: `binding`/`form` tags, defaults, `conform`, error bodies unchanged.
- **Outputs.** `return api.OK(v)` (status = `Doc.Status`, 200 when zero); several success statuses →
  `*api.Reply[T]` + `api.Respond(status, v)`; no body → `*api.Empty` + `api.Done()`.
- **Errors.** `return nil, api.Fail(status, coreErrors.BuildError…(c, …))`; `c := api.Gin(ctx)` is the Gin
  context for tenant/user/lang and the builders. A helper that used to write and return `false` now
  returns an `error` (`api.Fail`), the caller `return nil, err`.
- **Doc.** `Summary` (one line), `Description`, `Status`, `Errors` (the error statuses it answers — each
  documented as `core.ErrorResponse`), `Public: true` when it takes no bearer token.
- **Not huma:** the WebSocket, the jobs UI, and a route that streams or sets raw bytes — plain Gin, and
  the map lists them as undocumented.

## REST conventions

`/api/v1/{resource}` plural kebab-case · `Authorization: Bearer` · `X-Tenant-Slug` · errors via
`BuildErrorSingle` (one code) / `BuildErrorDetail` (binding, field detail) / `BuildError` (unexpected).

**The contract is `docs/openapi.json`** (served at `/docs` outside production): every operation's params,
body and response types, generated from the handlers (INFRA-021). `docs/routes.json` adds the SPs each one
reaches and the tables they write; `spec-agent/specs/context/system-map.md` adds who calls it. The gate's
`contract` stage fails when either file is stale, and when a change breaks a client against HEAD's
contract unless `docs/breaking.allow` lists the operation.

**List endpoints have no uniform shape — read the operation in the contract before writing a client.**
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
| `GET /tracking/documents` | `page page_size subject_type subject_id expiring_within_days search status` (cap 100) | `{ documents, summary, total_count, page, page_size }` — every row carries `type_name`, `subject_name` and a `status` of valid/expiring/expired; `search` is type name, subject name or number; `summary {expired, expiring, valid}` counts every filter but `status`, an empty page included (TRACK-076) |
| `GET /tracking/document-types` | `applies_to is_active` | array — the tenant's whole policy, unpaginated |
| `GET /tracking/organizations` | `page page_size search kind is_active` (cap 100) | `{ organizations, total_count, page, page_size }` — every row carries `my_role, my_groups` (the reader's membership, null for staff; an organization user lists only its own, TRACK-067) |
| `GET /tracking/organizations/:id/invites` | `status=pending\|sent\|accepted\|expired page page_size` (cap 100) | `{ invites, total_count, page, page_size }` — the guardian accounts of its riders with `riders`, the invitation's `channel, status, sent_at` (null without one) and `message` (TRACK-062) |
| `GET /tracking/stop-places` | `page page_size search near radius_m` (cap 100) | `{ stop_places, total_count, page, page_size }` — `near=lat,lng` narrows to `radius_m` metres (default 1000), adds `distance_m` to every row and orders nearest first |
| `GET /tracking/routes/:id/stops` | `date` (YYYY-MM-DD, default today) | array — the stops of the route version in force that day; every row carries `version_id` and `stop_place_id` |
| `GET /tracking/routes/:id/versions` | — | array, newest first — each version with `effective_from`, `effective_to` and `stops_count` |
| `GET /tracking/calendars` | `page page_size search` (cap 100) | `{ calendars, total_count, page, page_size }` — every row carries `dates_count` and `scope`: `schedule` (read by the schedules pointing at it), `tenant` (every route), `organization` (every route of that client); the preview and the materialiser skip a `no_service` day of any of the three (TRACK-076 D1) |
| `GET /tracking/calendar-dates` | `from to` (YYYY-MM-DD, required) | `{ dates }`, oldest first — the `tenant` and `organization` calendars' dates with `calendar_id, calendar_name, scope, organization_id, organization_name`; `POST { from, to ≤ 92 days, kind, label, organization_id? }` → 201 `{ calendar_id, added }` creates the scope's calendar on its first date; `DELETE /:calendar_id/:date` → 204; both write `route.changed` for every route the scope reaches |
| `GET /tracking/calendars/:id/dates` | `from to` (YYYY-MM-DD, both optional) | array, oldest first — `date`, `kind` of no_service/special_service, `label`; `PUT` replaces the whole list with a bare array of the same shape |
| `GET /tracking/routes/:id/schedules` | — | array, newest validity first — `days_of_week` is a bitmask (bit 0 Monday), `start_time` is HH:MM, every row carries `calendar_name` |
| `GET /tracking/routes/:id/exceptions` | `date_from date_to` (overlap, not containment) | array, oldest first — `kind` and a `payload` whose keys the kind defines |
| `GET /tracking/routes/:id/preview` | `from to` (YYYY-MM-DD, both required, at most 92 days apart) | array — one row per departure the plan produces, with `service_date`, `schedule_id`, `start_time`, `version_id`, `vehicle_id`, `driver_id`, a `status` of planned/cancelled, the `exceptions` that apply and `stops_count` — the day's stop list after its skip_stop/add_stop/detour (TRACK-065); a day the route does not run has no row |
| `GET /tracking/assignments` | `page page_size rider_id route_id date` (cap 100) | `{ assignments, total_count, page, page_size }` — `date` narrows to the assignments in force that day; every row carries `rider_name`, `route_name`, `direction`, `pickup_stop_name`, `dropoff_stop_name`; `days_of_week` is the schedules' bitmask. Writes (`POST`, `PATCH /:id`, `POST /bulk`) answer `{ assignment \| assignments, warnings }` |
| `GET /tracking/trips` | `page page_size date route_id status organization_id` (cap 100) | `{ trips, total_count, page, page_size }` — `date` defaults to each route's local today; every row carries `route_name`, `direction`, `license_plate`, `driver_name`, `stops_count`, `tasks_total` (not cancelled), `tasks_done` (done or no_show), `delay_seconds` (list only: next pending stop overdue by, 0 not due, null once ended), `service_block {subject: vehicle|driver, reason} | null` (an expired blocking document, read per row — also on `/mobile/driver/today`'s trips; TRACK-064); `GET /:id` and every trip write answer the row plus `stops[].tasks[]`; `GET /:id` also `polyline`, `distance_km`, `trace_source` (null until the trip completes) |
| `GET /tracking/alerts` | `page page_size route_id status severity` (cap 100) | `{ alerts, total_count, page, page_size }` — newest first, `status` active (default)/resolved/all; every row carries `route_name`, `license_plate`, `trip_id`+`lat`/`lng` (driver incidents), `created_by_name`, `resolved_by_name`. `PATCH /:id/resolve` answers the row; 404 `alert.not-found`, 409 `alert.already-resolved`. Raise and resolve write `alert.changed` → an `alert` frame on `fleet:{tenant}` (and `trip:{id}`) |
| `GET /tracking/reports/attendance` | `from to organization_id route_id rider_id page page_size` (cap 100) | `{ rows, summary, total_count, page, page_size }` — a row per rider `{ rider_id, rider_name, organization_name, days, boarded, no_shows, absent, cancelled, attendance_pct }` (boarded over boarded + no-shows + absent), least attended first; `summary { riders, days, boarded, no_shows, absent, attendance_pct }` |
| `GET /tracking/reports/punctuality` | `from to route_id organization_id page page_size` (cap 100) | same envelope — a row per route `{ route_id, route_name, direction, organization_name, trips, on_time, on_time_pct, avg_start_delay_min, max_start_delay_min, avg_stop_delay_min, worst_stop {stop_name, p90_delay_min} \| null }` over completed trips (on time: start ≤ 5 min late and no stop > 5 min late), least punctual first |
| `GET /tracking/reports/fleet-usage` | `from to route_id organization_id group_by=vehicle\|driver page page_size` (cap 100) | same envelope — a row per vehicle or driver `{ id, label, company_name, trips, km, hours, riders_boarded, days_active, avg_occupancy_pct }`, most km first; `summary { vehicles \| drivers, trips, km, hours, riders_boarded }` |
| `GET /tracking/reports/incidents` | `from to route_id organization_id alert_type page page_size` (cap 100) | same envelope — `GET /tracking/alerts` rows raised in the range (UTC days), newest first; `summary { total, open, by_type {delay, breakdown, cancellation, emergency, other}, avg_resolve_min }` |
| `GET /tracking/trips/:id/events` | — | array, oldest first — `{ id, type, payload, created_at, created_by_name }`, `type` = the transition, `override` or `incident` |
| `GET /tracking/live/fleet` | `organization_id` | `{ vehicles: [{ vehicle_id, license_plate, trip_id, route_name, lat, lng, speed, heading, recorded_at }] }` — every vehicle that has reported, unpaginated; the org filter keeps vehicles whose trip carries its riders |
| `GET /tracking/trips/:id/live` | — | the `GET /:id` detail plus `position` (the last position frame, or null) and `eta: [{ trip_stop_id, eta_at, estimate_source }]` per pending stop, empty unless the trip is in progress and has a position |
| `GET /tracking/riders` | `page page_size search organization_id rider_type is_active` (cap 100) | `{ riders, total_count, page, page_size }` — every row carries `organization_name`; every rider shape carries `home_latitude`, `home_longitude`, `notes` |
| `GET /tracking/riders/:id/absences` | `from to` (overlap, both optional) | array, oldest first — `direction` null is both, `reported_via` web/portal, `reported_by_name`; on the portal chain (a guardian sees own riders). `POST` answers `{ absence, cancelled_tasks }`; 409 `overlap`, `cutoff` (guardian inside the organization's `absence_cutoff_min`) |
| `GET /tracking/riders/:id/audit` | `from to page page_size` (cap 100) | `{ entries, total_count, page, page_size }` — newest first, staff only: one entry per guardian (`level` portal) or organization user and `action` (status/today/live/absences) per hour, `reads` the calls in it, `email first_name last_name` null once the account is gone; the nightly `tracking:contact-retention` (02:00 UTC) prunes it past `TRACKING_ACCESS_LOG_RETENTION_DAYS` and clears inactive riders' contact data past the tenant's `settings["tracking.contact_retention_days"]`, else `TRACKING_CONTACT_RETENTION_DAYS` (TRACK-063) |
| `GET /mobile/portal/me` | — (Bearer only, no `X-Tenant-Slug`) | `{ user: { id, first_name, last_name }, tenants: [{ slug, name, riders, error }] }` — every tenant where the account is `portal`, newest membership first, `riders` as `GET /tracking/riders/today` answers them (which also carry `organization_kind`, `rider_type`); one operator failing is `error: true, riders: null`; no portal membership → 403 (TRACK-061) |
| `GET /tracking/riders/:id/today` | — (`If-None-Match`) | `{ rider_id, rider_name, date, timezone, organization, company { name, phone }, absence, trips: [{ …, pickup, dropoff, stops_left }] }` — every trip of each route's local today carrying the rider, by `planned_start`; `date` is the organization's today; `pickup`/`dropoff` carry `planned_at arrived_at task_status done_at eta_at`, `eta_at` only on a pending stop of the trip in progress while the rider rides it; `ETag` = sha256 of the body → 304; portal chain |
| `GET /tracking/riders/:id/history` | `from to` (default the 30 days to the rider's local yesterday, ≤ 31) | `{ rider_id, from, to, days: [{ date, status, legs, absence }] }` newest first, unpaginated — `status` absent > no_show > rode > partial > cancelled (anything that carried them nowhere); days with neither leg nor absence left out; 400 `tracking.rider.history-range`; portal chain |
| `GET /tracking/riders/:id/absence-cutoffs` | `from to` (default today + 13, ≤ 31) | `{ cutoff_min, days: [{ date, has_trip, cutoff_at, closed }] }` — the first pickup less `absence_cutoff_min`, the rule `POST …/absences` answers 409 `cutoff` on; 400 `tracking.rider.cutoffs-range`; portal chain |
| `GET /tracking/riders/:id/suggestions` | `direction` (required) `days` (bitmask, 127) `max_walk_m` (800, ≤ 3000) `limit` (10, ≤ 50) | `{ suggestions }` — straight line from `home_location`, nearest first then most `free_seats`; 409 `tracking.rider.no-home-location` |
| `GET /sales/orders` | `limit offset` | `{ data }` — no total |
| `GET /sales/customers`, the other `/tracking/*`, `/inventory/stock/:id` | filters only | array, object or `{ data }`, unpaginated |

New list endpoints use the purchasing shape. No total ⇒ the app can only "load more"; no `search`
param ⇒ search is client-side or new API work — the spec decides, never the implementation.

## Adding a module

1. Layout above · 2. `config/config.go` + register in `ModularAppConfig` · 3. migrations + `sql/functions` · 4. `lang/` ·
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

`valhalla` serves routing from `${GEO_DATA_DIR}/current`; the basemap is on R2 behind
`tiles.taypi24.com` (INFRA-008). Builds are made off the server: `docker/geo/publish.sh` puts the
basemap live, `docker/geo/switch.sh` routing and places (`docker/geo/README.md`). The
default stack is the whole product on one box (INFRA-006): `api-realtime` runs in platform mode and
Caddy routes `PLATFORM_HOST/api/v1/tracking/ws` to it; the `tenant` profile is a tenant-mode server
for a dedicated box.

`docker/backup/README.md` has the nightly dump and the restore steps; deploy variables are at the
bottom of `.env.example`. Point-in-time recovery is pgBackRest inside the `postgres` image
(INFRA-017: WAL to R2 within a minute, `docker/restore.sh info|drill|restore`); alerting is INFRA-018;
PgBouncer has no spec yet.

**One box (INFRA-004).** `docker/README.md` is the server runbook: the memory split of the 12 GB, the
connection budget (`DATABASE_POOL_SIZE` + `TENANCY_DATABASE_POOL_SIZE` per service, idle connections
close after 5 min, `max_connections=200` covers 7 concurrently active tenants), the Postgres settings on
the compose command line, log caps (json-file, 5 × 20 MB per container), the Debian host prep and
**First deploy** — the ordered runbook from an empty box to `api.taypi24.com`. Every
`api-*` runs `APP_MODE=release` with a `mem_limit` and `GOMEMLIMIT`; `LOG_LEVEL` (info) comes from the
compose `.env`, `LOG_FORMAT` (text|json) from the server env file. Per-event lines — socket open/close,
outbox purge, partitions, one translation file — are debug; failures stay at info or above.

## Gotchas not visible in code

- Function files run after their module's migrations, all of a module in one transaction: one failing
  file leaves the module's functions as they were and the server retries like a dirty migration. A
  fresh database still runs every old migration, old function versions included, and the files then
  overwrite them. On that fresh database a migration calling another module's SP gets the
  SP's current version, not the one of the migration's day.
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
- `.env.platform` has SMTP off, so OTP requests answer 503. To sign in by code locally, point `SMTP_*`
  at any sink that accepts `AUTH` (`net/smtp` refuses a server without it) and start with
  `SMTP_ENABLED=true` in the environment — env wins over the file.
- In `modules/tracking/tests/` and `modules/geo/tests/` a test file must sort after `helpers.go`
  (`helpers.go` declares the `_test` package; an earlier `_test.go` file is read as an external test and
  the package fails to load).
