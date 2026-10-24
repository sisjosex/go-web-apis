.PHONY: help contract build run \
        dev seed-dev \
        migrate migrate-list tenant-list tenant-migrate \
        test test-auth test-users test-core test-tenancy test-tracking test-inventory test-sales test-purchasing test-billing test-import test-geo test-all \
        geo-build geo-switch geo-rollback geo-up \
        db-reset schema docker-up docker-down clean

# Build output
BINARY := bin/server
GOFLAGS := -v

# ════════════════════════════════════════════════════════════════
# HELP
# ════════════════════════════════════════════════════════════════
help:
	@echo "════════════════════════════════════════════════════════════════"
	@echo "  Go Web API - Makefile Commands"
	@echo "════════════════════════════════════════════════════════════════"
	@echo ""
	@echo "🗄️  MIGRATIONS  (MODULE and NAME required)"
	@echo "  make migrate MODULE=auth NAME=add_field   - Create migration files"
	@echo "  make migrate-list                         - List available modules"
	@echo ""
	@echo "🏢 TENANTS  (requires .env.platform with TENANCY_ENABLED=true)"
	@echo "  make tenant-list                          - List tenants with custom DBs"
	@echo "  make tenant-migrate SLUG=acme             - Migrate a specific tenant"
	@echo "  make tenant-migrate SLUG=all              - Migrate all tenants"
	@echo ""
	@echo "📚 CONTRACT & BUILD"
	@echo "  make contract         - Write docs/openapi.json and docs/routes.json from the routes' types (commit both)"
	@echo "  make build            - Build server binary"
	@echo ""
	@echo "✅ QUALITY GATE"
	@echo "  make gate             - fmt + lint + check + build, then each changed module's tests + route coverage, the contract, then the schema snapshot"
	@echo "                          (MODULES=\"tracking inventory\" overrides the modules picked from git diff)"
	@echo "  make lint             - golangci-lint on changed code only (--new); make lint-all for the tree"
	@echo "  make check            - tools/check: gofmt, tenant scope in SPs, DTO tenant binding (changed files)"
	@echo ""
	@echo "🚀 RUN  (compiled binary)"
	@echo "  make run              - Start server (port 8080)"
	@echo ""
	@echo "💻 DEV"
	@echo "  make dev              - Start development server"
	@echo ""
	@echo "🧪 TESTS"
	@echo "  make test             - Reset DB + run main test suite"
	@echo "  make test-all         - Reset DB + run all module tests + every module's route coverage"
	@echo "  make test-auth        - Auth module"
	@echo "  make test-tenancy     - Tenancy module"
	@echo "  make test-inventory   - Inventory module"
	@echo "  make test-sales       - Sales module"
	@echo "  make test-purchasing  - Purchasing module"
	@echo "  make test-billing     - Billing module"
	@echo "  make test-tracking    - Tracking module"
	@echo "  make test-users       - Users module"
	@echo "  make test-core        - Core module (jobs tests need Valkey, storage tests the local S3)"
	@echo "  make test-import      - Import engine (unit) + its routes through users and inventory"
	@echo "  make test-geo         - Geo module (address search, routing breaker, ETA fallback)"
	@echo ""
	@echo "🗄️  DATABASE"
	@echo "  make db-reset         - Drop, recreate, and migrate test database"
	@echo "  make schema           - Reset, then rewrite every modules/<m>/sql/schema.sql"
	@echo ""
	@echo "🐳 DOCKER"
	@echo "  make docker-up        - Start docker-compose services (PostgreSQL, Valkey)"
	@echo "  make docker-down      - Stop docker-compose services"
	@echo ""
	@echo "🗺️  GEO  (docker/geo/README.md)"
	@echo "  make geo-build [DATE=YYYY-MM-DD]   - Build routing, basemap, places into docker/geo/data/<date>"
	@echo "  make geo-switch DATE=YYYY-MM-DD    - Check that build, make it current, import places"
	@echo "  make geo-rollback                  - Swap current and previous"
	@echo "  make geo-up                        - Start valhalla (:8002), vroom (:3000) and the tiles server (:8090)"
	@echo ""
	@echo "🧹 CLEANUP"
	@echo "  make clean            - Remove built binary"
	@echo "════════════════════════════════════════════════════════════════"

# ════════════════════════════════════════════════════════════════
# MIGRATIONS
# ════════════════════════════════════════════════════════════════
migrate:
	@[ "$(MODULE)" ] || (echo "❌ MODULE required. Usage: make migrate MODULE=auth NAME=add_field"; exit 1)
	@[ "$(NAME)" ]   || (echo "❌ NAME required.   Usage: make migrate MODULE=auth NAME=add_field"; exit 1)
	go run ./cmd/cli m -module=$(MODULE) -name=$(NAME)

migrate-list:
	go run ./cmd/cli m -list

# ════════════════════════════════════════════════════════════════
# TENANT MANAGEMENT
# ════════════════════════════════════════════════════════════════
tenant-list:
	go run ./cmd/cli t -list

tenant-migrate:
	@[ "$(SLUG)" ] || (echo "❌ SLUG required. Usage: make tenant-migrate SLUG=acme  (or SLUG=all)"; exit 1)
	go run ./cmd/cli t -migrate=$(SLUG)

# ════════════════════════════════════════════════════════════════
# CONTRACT (INFRA-021)
# ════════════════════════════════════════════════════════════════
# docs/openapi.json is built from the huma operations' types by mounting every route as the tests do
# (the test database must exist); docs/routes.json adds the stored procedures each one reaches.
contract:
	go test ./tools/contract -run TestContract -count=1 -args -write
	go run ./tools/routemap

# The gate's stage: both files current, and no client broken against HEAD's contract.
CONTRACT_CHECK := { go test ./tools/contract -run TestContract -count=1 && go run ./tools/routemap -check && go run ./tools/breaking; }

# ════════════════════════════════════════════════════════════════
# BUILD
# ════════════════════════════════════════════════════════════════
build:
	@echo "🔨 Building server..."
	go build $(GOFLAGS) -o $(BINARY) ./cmd/server
	@echo "✅ Binary: $(BINARY)"

# ════════════════════════════════════════════════════════════════
# RUN (compiled)
# ════════════════════════════════════════════════════════════════
run: build
	@echo "🚀 Server → http://localhost:8080  |  Contract → http://localhost:8080/docs"
	./$(BINARY)

# ════════════════════════════════════════════════════════════════
# DEV
# ════════════════════════════════════════════════════════════════
dev:
	@echo "💻 Dev mode → http://localhost:8080"
	go run ./cmd/server

# The emulator's dev accounts (cmd/cli/seed_dev.sql) in the .env.platform database; idempotent.
seed-dev:
	go run ./cmd/cli seed-dev

# ════════════════════════════════════════════════════════════════
# TESTS
# ════════════════════════════════════════════════════════════════
TEST_FLAGS := -tags=integration $(GOFLAGS) -timeout=120s -p 1
# The test engine records every request here; `go run ./tools/check -routes` reads it (INFRA-014).
ROUTE_HITS := $(CURDIR)/.route-hits
TEST_MODULES := auth users import core tenancy tracking inventory sales purchasing billing geo
GO_TEST := rm -f $(ROUTE_HITS); ROUTE_HITS=$(ROUTE_HITS) go test $(TEST_FLAGS)

test: db-reset
	$(GO_TEST) \
		./modules/auth/tests \
		./modules/tenancy/tests \
		./modules/tenancy/middleware \
		./modules/core/services/... \
		./modules/inventory/tests \
		./modules/sales/tests

test-all: db-reset
	@echo "🧪 Running all module tests..."
	$(GO_TEST) \
		./modules/auth/tests ./modules/auth/services/... \
		./modules/users/tests ./modules/users/services \
		./modules/tenancy/tests \
		./modules/tenancy/middleware \
		./modules/core/tests ./modules/core/services/... ./modules/core/utils \
		./modules/import/services \
		./modules/tracking/tests \
		./modules/inventory/tests ./modules/inventory/services \
		./modules/sales/tests \
		./modules/purchasing/tests \
		./modules/billing/tests \
		./modules/geo/tests ./modules/geo/services/routing
	go run ./tools/check -routes $(ROUTE_HITS) -module "$(TEST_MODULES)"
	@echo "✅ All tests done"

test-auth: db-reset
	$(GO_TEST) ./modules/auth/tests ./modules/auth/services/...

test-users: db-reset
	$(GO_TEST) ./modules/users/tests ./modules/users/services

# The engine's HTTP routes are driven by the resources that plug into it: users and inventory.
test-import: db-reset
	$(GO_TEST) ./modules/import/services ./modules/users/tests ./modules/inventory/tests

test-core: db-reset
	$(GO_TEST) ./modules/core/tests ./modules/core/services/... ./modules/core/utils

test-tenancy: db-reset
	$(GO_TEST) ./modules/tenancy/tests ./modules/tenancy/middleware

test-tracking: db-reset
	$(GO_TEST) ./modules/tracking/tests

test-inventory: db-reset
	$(GO_TEST) ./modules/inventory/tests ./modules/inventory/services

test-sales: db-reset
	$(GO_TEST) ./modules/sales/tests

test-purchasing: db-reset
	$(GO_TEST) ./modules/purchasing/tests

test-billing: db-reset
	$(GO_TEST) ./modules/billing/tests

test-geo: db-reset
	$(GO_TEST) ./modules/geo/tests ./modules/geo/services/routing

# ════════════════════════════════════════════════════════════════
# DATABASE
# ════════════════════════════════════════════════════════════════
db-reset:
	@echo "🔄 Resetting test database..."
	go run ./cmd/testutil/main.go ./cmd/testutil/seed.go -reset

# The schema snapshot (INFRA-015): tables, indexes and views of every module, never a function.
# pg_dump runs inside the test database's container, so its version matches the server's.
PG_DUMP_CONTAINER ?= tenant-db
SCHEMA_DUMP := PG_DUMP_CONTAINER=$(PG_DUMP_CONTAINER) go run ./cmd/cli schema dump
# The gate compares the worktree snapshot before and after the dump (CR ignored: the checkout is CRLF).
SCHEMA_SUMS := for f in modules/*/sql/schema.sql; do [ -f "$$f" ] && echo "$$f $$(tr -d '\r' < "$$f" | sha256sum | cut -c1-16)"; done

schema: db-reset
	$(SCHEMA_DUMP)

# ════════════════════════════════════════════════════════════════
# DOCKER
# ════════════════════════════════════════════════════════════════
docker-up:
	@echo "🐳 Starting Docker services..."
	docker-compose up -d
	@echo "✅ Services started. Check: docker-compose ps"

docker-down:
	@echo "🛑 Stopping Docker services..."
	docker-compose down
	@echo "✅ Services stopped"

# ════════════════════════════════════════════════════════════════
# GEO (INFRA-003) — the dev side of docker/geo; on the server the scripts run
# directly against docker-compose.prod.yml (docker/geo/README.md)
# ════════════════════════════════════════════════════════════════
GEO_DEV := GEO_COMPOSE=docker-compose.yml ENV_FILE=.env.platform \
	GEO_IMPORT="go run ./cmd/cli geo import docker/geo/data/current/places.geojsonl"

geo-build:
	docker/geo/build.sh $(DATE)

geo-switch:
	@[ "$(DATE)" ] || (echo "❌ DATE required. Usage: make geo-switch DATE=2026-09-23"; exit 1)
	$(GEO_DEV) docker/geo/switch.sh $(DATE)

geo-rollback:
	$(GEO_DEV) docker/geo/rollback.sh

geo-up:
	docker compose --profile geo up -d valhalla vroom tiles

# ════════════════════════════════════════════════════════════════
# QUALITY GATE — quiet: one line per step, the tail of the log on failure
# ════════════════════════════════════════════════════════════════
GATE_LOG := .gate.log
# run a step quietly: $(call quiet,<name>,<command>)
define quiet
	@printf "> %s " "$(1)"; start=$$(date +%s); \
	if $(2) > $(GATE_LOG) 2>&1; then echo "ok ($$(( $$(date +%s) - start ))s)"; \
	else echo "FAIL"; tail -40 $(GATE_LOG); echo "GATE FAILED at $(1)"; rm -f $(GATE_LOG); exit 1; fi
endef

lint:
	golangci-lint run --new ./...

lint-all:
	golangci-lint run ./...

check:
	go run ./tools/check

check-all:
	go run ./tools/check -all

gate:
	$(call quiet,check,go run ./tools/check)
	$(call quiet,lint,golangci-lint run --new ./...)
	$(call quiet,build,go build -o $(BINARY) ./cmd/server)
	@mods="$(MODULES)"; [ -n "$$mods" ] || mods=$$(go run ./tools/check -modules); \
	echo "> modules: $${mods:-none}"; \
	for m in $$mods; do \
		printf "> test-$$m "; start=$$(date +%s); \
		if $(MAKE) -s test-$$m > $(GATE_LOG) 2>&1 && go run ./tools/check -routes $(ROUTE_HITS) -module $$m >> $(GATE_LOG) 2>&1; \
		then echo "ok ($$(( $$(date +%s) - start ))s)"; \
		else echo "FAIL"; grep -E "^(---|===|FAIL|panic|\s+[a-z_]+\.go:[0-9]+|unmatched|route without|check -routes)" $(GATE_LOG) | grep -vE "^(--- PASS|=== (RUN|PAUSE|CONT))" | tail -40; echo "GATE FAILED at test-$$m"; rm -f $(GATE_LOG); exit 1; fi; \
	done; \
	printf "> contract "; start=$$(date +%s); \
	if $(CONTRACT_CHECK) > $(GATE_LOG) 2>&1; then echo "ok ($$(( $$(date +%s) - start ))s)"; \
	else echo "FAIL"; grep -E "stale|breaking|routes registered|FAIL|Error" $(GATE_LOG) | tail -40; echo "GATE FAILED at contract"; rm -f $(GATE_LOG); exit 1; fi; \
	[ -z "$$mods" ] || { printf "> schema "; before=$$($(SCHEMA_SUMS)); \
		if $(SCHEMA_DUMP) > $(GATE_LOG) 2>&1 && [ "$$before" = "$$($(SCHEMA_SUMS))" ]; \
		then echo "ok"; \
		else echo "FAIL"; tail -5 $(GATE_LOG); echo "$$before" > $(GATE_LOG); \
			{ $(SCHEMA_SUMS); } | grep -vxFf $(GATE_LOG) | cut -d' ' -f1; \
			echo 'schema.sql no longer matches the migrations: run `make schema` and commit it'; \
			echo "GATE FAILED at schema"; rm -f $(GATE_LOG); exit 1; fi; }
	@rm -f $(GATE_LOG); echo "GATE PASSED"

# ════════════════════════════════════════════════════════════════
# CLEANUP
# ════════════════════════════════════════════════════════════════
clean:
	@echo "🧹 Removing binaries..."
	@rm -rf bin/
	@echo "✅ Done"

.DEFAULT_GOAL := help
