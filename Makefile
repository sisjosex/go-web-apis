.PHONY: help swagger build run \
        dev \
        migrate migrate-list tenant-list tenant-migrate \
        test test-auth test-users test-core test-tenancy test-tracking test-inventory test-sales test-purchasing test-billing test-import test-geo test-all \
        geo-build geo-switch geo-rollback geo-up \
        db-reset docker-up docker-down clean

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
	@echo "📚 SWAGGER & BUILD"
	@echo "  make swagger          - Generate Swagger documentation"
	@echo "  make build            - Build server binary"
	@echo ""
	@echo "✅ QUALITY GATE"
	@echo "  make gate             - fmt + lint + check + build, quiet; MODULES=\"tracking inventory\" adds their tests"
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
	@echo "  make test-all         - Reset DB + run all module tests"
	@echo "  make test-auth        - Auth module"
	@echo "  make test-tenancy     - Tenancy module"
	@echo "  make test-inventory   - Inventory module"
	@echo "  make test-sales       - Sales module"
	@echo "  make test-purchasing  - Purchasing module"
	@echo "  make test-billing     - Billing module"
	@echo "  make test-tracking    - Tracking module"
	@echo "  make test-users       - Users module"
	@echo "  make test-core        - Core module (jobs tests need Valkey up)"
	@echo "  make test-import      - Import engine (unit, no DB)"
	@echo "  make test-geo         - Geo module (address search, routing breaker, ETA fallback)"
	@echo ""
	@echo "🗄️  DATABASE"
	@echo "  make db-reset         - Drop, recreate, and migrate test database"
	@echo ""
	@echo "🐳 DOCKER"
	@echo "  make docker-up        - Start docker-compose services (PostgreSQL, Valkey)"
	@echo "  make docker-down      - Stop docker-compose services"
	@echo ""
	@echo "🗺️  GEO  (docker/geo/README.md)"
	@echo "  make geo-build [DATE=YYYY-MM-DD]   - Build routing, basemap, places into docker/geo/data/<date>"
	@echo "  make geo-switch DATE=YYYY-MM-DD    - Check that build, make it current, import places"
	@echo "  make geo-rollback                  - Swap current and previous"
	@echo "  make geo-up                        - Start valhalla (:8002) and the tiles server (:8090)"
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
# SWAGGER
# ════════════════════════════════════════════════════════════════
swagger:
	@echo "🔄 Generating Swagger documentation..."
	@if command -v swag > /dev/null 2>&1; then \
		swag init -g cmd/server/main.go -d . --parseDependency --parseInternal --parseDepth 5 -o docs 2>&1 | grep -v "warning\|ParseComment"; \
		[ -f docs/swagger.json ] && echo "✅ Swagger docs generated → http://localhost:8080/swagger/index.html" || echo "❌ docs/swagger.json not generated"; \
	else \
		echo "❌ swag not installed. Run: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

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
	@echo "🚀 Server → http://localhost:8080  |  Swagger → http://localhost:8080/swagger/index.html"
	./$(BINARY)

# ════════════════════════════════════════════════════════════════
# DEV
# ════════════════════════════════════════════════════════════════
dev:
	@echo "💻 Dev mode → http://localhost:8080"
	go run ./cmd/server

# ════════════════════════════════════════════════════════════════
# TESTS
# ════════════════════════════════════════════════════════════════
TEST_FLAGS := -tags=integration $(GOFLAGS) -timeout=120s -p 1

test: db-reset
	go test $(TEST_FLAGS) \
		./modules/auth/tests \
		./modules/tenancy/tests \
		./modules/tenancy/middleware \
		./modules/core/services \
		./modules/inventory/tests \
		./modules/sales/tests

test-all: db-reset
	@echo "🧪 Running all module tests..."
	go test $(TEST_FLAGS) \
		./modules/auth/tests \
		./modules/users/tests \
		./modules/tenancy/tests \
		./modules/tenancy/middleware \
		./modules/core/services \
		./modules/tracking/tests \
		./modules/inventory/tests \
		./modules/sales/tests \
		./modules/purchasing/tests \
		./modules/billing/tests
	@echo "✅ All tests done"

test-auth: db-reset
	go test $(TEST_FLAGS) ./modules/auth/tests

test-users: db-reset
	go test $(TEST_FLAGS) ./modules/users/tests ./modules/users/services

test-import:
	go test $(TEST_FLAGS) ./modules/import/services

test-core: db-reset
	go test $(TEST_FLAGS) ./modules/core/tests ./modules/core/services

test-tenancy: db-reset
	go test $(TEST_FLAGS) ./modules/tenancy/tests ./modules/tenancy/middleware

test-tracking: db-reset
	go test $(TEST_FLAGS) ./modules/tracking/tests

test-inventory: db-reset
	go test $(TEST_FLAGS) ./modules/inventory/tests

test-sales: db-reset
	go test $(TEST_FLAGS) ./modules/sales/tests

test-purchasing: db-reset
	go test $(TEST_FLAGS) ./modules/purchasing/tests

test-billing: db-reset
	go test $(TEST_FLAGS) ./modules/billing/tests

test-geo: db-reset
	go test $(TEST_FLAGS) ./modules/geo/tests ./modules/geo/services/routing

# ════════════════════════════════════════════════════════════════
# DATABASE
# ════════════════════════════════════════════════════════════════
db-reset:
	@echo "🔄 Resetting test database..."
	go run ./cmd/testutil/main.go ./cmd/testutil/seed.go -reset

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
GEO_DEV := GEO_COMPOSE=docker-compose.yml ENV_FILE=.env.tenant \
	GEO_IMPORT="go run ./cmd/cli geo import docker/geo/data/current/places.geojsonl"

geo-build:
	docker/geo/build.sh $(DATE)

geo-switch:
	@[ "$(DATE)" ] || (echo "❌ DATE required. Usage: make geo-switch DATE=2026-09-23"; exit 1)
	$(GEO_DEV) docker/geo/switch.sh $(DATE)

geo-rollback:
	$(GEO_DEV) docker/geo/rollback.sh

geo-up:
	docker compose --profile geo up -d valhalla tiles

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
	@for m in $(MODULES); do \
		printf "> test-$$m "; start=$$(date +%s); \
		if $(MAKE) -s test-$$m > $(GATE_LOG) 2>&1; then echo "ok ($$(( $$(date +%s) - start ))s)"; \
		else echo "FAIL"; grep -E "^(---|===|FAIL|panic|\s+[a-z_]+\.go:[0-9]+)" $(GATE_LOG) | tail -40; echo "GATE FAILED at test-$$m"; rm -f $(GATE_LOG); exit 1; fi; \
	done
	@rm -f $(GATE_LOG); echo "GATE PASSED"

# ════════════════════════════════════════════════════════════════
# CLEANUP
# ════════════════════════════════════════════════════════════════
clean:
	@echo "🧹 Removing binaries..."
	@rm -rf bin/
	@echo "✅ Done"

.DEFAULT_GOAL := help
