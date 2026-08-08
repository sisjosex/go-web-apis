.PHONY: help swagger build run \
        dev \
        migrate migrate-list tenant-list tenant-migrate \
        test test-auth test-users test-core test-tenancy test-tracking test-inventory test-sales test-purchasing test-billing test-import test-all \
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
	@echo "  make test-core        - Core module"
	@echo "  make test-import      - Import engine (unit, no DB)"
	@echo ""
	@echo "🗄️  DATABASE"
	@echo "  make db-reset         - Drop, recreate, and migrate test database"
	@echo ""
	@echo "🐳 DOCKER"
	@echo "  make docker-up        - Start docker-compose services"
	@echo "  make docker-down      - Stop docker-compose services"
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
# CLEANUP
# ════════════════════════════════════════════════════════════════
clean:
	@echo "🧹 Removing binaries..."
	@rm -rf bin/
	@echo "✅ Done"

.DEFAULT_GOAL := help
