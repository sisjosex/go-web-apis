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
make docker-up | docker-down          # PostgreSQL containers (tests need them up)
make test-<module>                    # auth core users tenancy tracking inventory sales purchasing billing
make test-all                         # every module (db-reset first)
make migrate MODULE=x NAME=y          # .up.sql + .down.sql pair
make tenant-migrate SLUG=x            # apply to one tenant DB
make swagger                          # regenerate docs
```

Setup is human-only: copy `.env.example` → `.env.platform` / `.env.tenant`, then `make docker-up`.

## Shape

**Stack** Go 1.22+ · Gin · pgx/v5 · PostgreSQL · SQL migrations, no ORM.

| Server | Entry | Env | Port | Modules |
|---|---|---|---|---|
| Platform | `cmd/server -mode=platform` | `.env.platform` | 8080 | core auth users tenancy billing |
| Tenant | `cmd/server -mode=tenant` | `.env.tenant` | 9080 | core auth users tracking inventory sales purchasing |

One binary, one `-mode` flag: `cmd/platform` and `cmd/tenant` have not existed for some time. The
sibling `cmd/cli` carries `migration` (generate), `migrate` (apply, one shot) and `tenant`.

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
| `GET /tracking/organizations` | `page page_size search kind is_active` (cap 100) | `{ organizations, total_count, page, page_size }` |
| `GET /tracking/riders` | `page page_size search organization_id rider_type is_active` (cap 100) | `{ riders, total_count, page, page_size }` |
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
still migrate on start — one replica, nothing to race with. `/livez` (always 200) and `/readyz` (200
when the pool pings, else 503) back the healthchecks; both are registered before the CORS and rate
limit middleware, so a probe is never rate-limited.

Nothing in the API issues `CREATE DATABASE` — the tenancy service only migrates a URL it is handed.
The tenant server's own database is created by `docker/postgres/init-extra-databases.sh` from
`POSTGRES_EXTRA_DBS`, and a per-tenant database is created by hand before `cli tenant -migrate <slug>`.
The users module runs against tenant databases too, which carry `auth.*` but no `tenancy.*`: every
`tenancy` reference in a `users` migration must be guarded by `to_regclass`.

`docker/backup/README.md` has the nightly dump and the restore steps; deploy variables are at the
bottom of `.env.example`. PITR, PgBouncer and monitoring are INFRA-004.

## Gotchas not visible in code

- `make test-*` runs `db-reset` first: drops, recreates, migrates and seeds `josex_test`. A `db-reset`
  failure is a real current problem, not a known issue to step around.
- Adding a column to an SP without adding it to `row.Scan` fails silently at runtime, not at build.
- A test file without `//go:build integration` is excluded and the module looks green.
- `t.Skipf` on an unexpected status hid a 500-ing sales flow for months. Use `t.Fatalf`.
- Module error codes are matched by exact string in the app — renaming one is an app change too.
