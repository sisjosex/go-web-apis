.PHONY: help swagger build build-platform build-tenant run run-platform run-tenant dev dev-platform dev-tenant test test-auth test-users test-core test-tenancy test-tracking test-inventory test-sales test-all db-reset docker-up docker-down clean

# Build variables
BINARY_PLATFORM := bin/platform
BINARY_TENANT := bin/tenant
GOFLAGS := -v

help:
	@echo "════════════════════════════════════════════════════════════════"
	@echo "  Go Web API - Makefile Commands"
	@echo "════════════════════════════════════════════════════════════════"
	@echo ""
	@echo "📚 SWAGGER & BUILD:"
	@echo "  make swagger           - Generate Swagger documentation"
	@echo "  make build             - Build both platform and tenant binaries"
	@echo "  make build-platform    - Build platform server binary"
	@echo "  make build-tenant      - Build tenant server binary"
	@echo ""
	@echo "🚀 RUN SERVERS (Compiled):"
	@echo "  make run               - Run platform server (default)"
	@echo "  make run-platform      - Run platform server on port 8080"
	@echo "  make run-tenant        - Run tenant server on port 8081"
	@echo ""
	@echo "💻 DEVELOPMENT MODE (go run - Fast with auto-reload via air):"
	@echo "  make dev               - Run platform in dev mode"
	@echo "  make dev-platform      - Run platform in dev mode on port 8080"
	@echo "  make dev-tenant        - Run tenant in dev mode on port 8081"
	@echo ""
	@echo "🧪 TESTS:"
	@echo "  make test              - Run auth tests (default)"
	@echo "  make test-auth         - Run auth module tests"
	@echo "  make test-users        - Run users module tests"
	@echo "  make test-core         - Run core module tests"
	@echo "  make test-tenancy      - Run tenancy module tests"
	@echo "  make test-tracking     - Run tracking module tests"
	@echo "  make test-inventory    - Run inventory module tests"
	@echo "  make test-sales        - Run sales module tests"
	@echo "  make test-all          - Run all module tests"
	@echo ""
	@echo "🗄️  DATABASE:"
	@echo "  make db-reset          - Reset test database"
	@echo ""
	@echo "🐳 DOCKER:"
	@echo "  make docker-up         - Start docker-compose services"
	@echo "  make docker-down       - Stop docker-compose services"
	@echo ""
	@echo "🧹 CLEANUP:"
	@echo "  make clean             - Remove built binaries"
	@echo "════════════════════════════════════════════════════════════════"

# ════════════════════════════════════════════════════════════════
# SWAGGER
# ════════════════════════════════════════════════════════════════
swagger:
	@echo "🔄 Generating Swagger documentation..."
	@if command -v swag > /dev/null 2>&1; then \
		swag init -g cmd/platform/main.go -d . -o docs --parseInternal --parseDepth 3 2>&1 | grep -v "warning" || true; \
		if [ -f docs/swagger.json ]; then \
			echo "✅ Swagger generated successfully in ./docs/"; \
			echo "📚 Access at: http://localhost:8080/swagger/index.html"; \
		fi; \
	else \
		echo "❌ swag not installed. Install with:"; \
		echo "   go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# ════════════════════════════════════════════════════════════════
# BUILD
# ════════════════════════════════════════════════════════════════
build: build-platform build-tenant
	@echo "✅ Build complete! Binaries in ./bin/"

build-platform:
	@echo "🔨 Building platform server..."
	go build $(GOFLAGS) -o $(BINARY_PLATFORM) ./cmd/platform
	@echo "✅ Platform binary: $(BINARY_PLATFORM)"

build-tenant:
	@echo "🔨 Building tenant server..."
	go build $(GOFLAGS) -o $(BINARY_TENANT) ./cmd/tenant
	@echo "✅ Tenant binary: $(BINARY_TENANT)"

# ════════════════════════════════════════════════════════════════
# RUN SERVERS
# ════════════════════════════════════════════════════════════════
run: run-platform

run-platform: build-platform
	@echo "🚀 Starting platform server on http://localhost:8080"
	@echo "📚 Swagger docs: http://localhost:8080/swagger/index.html"
	@echo "Press Ctrl+C to stop"
	./$(BINARY_PLATFORM)

run-tenant: build-tenant
	@echo "🚀 Starting tenant server on http://localhost:8081"
	@echo "📚 Swagger docs: http://localhost:8081/swagger/index.html"
	@echo "Press Ctrl+C to stop"
	./$(BINARY_TENANT)

# ════════════════════════════════════════════════════════════════
# DEVELOPMENT MODE (go run - Hot reload with air)
# ════════════════════════════════════════════════════════════════
dev: dev-platform

dev-platform:
	@echo "💻 Starting platform in development mode..."
	@echo "🔄 With auto-reload support (install air first: go install github.com/cosmtrek/air@latest)"
	@echo "📚 Swagger docs: http://localhost:8080/swagger/index.html"
	@echo "Press Ctrl+C to stop"
	@if command -v air > /dev/null 2>&1; then \
		air -c .air.toml; \
	else \
		@echo "⚠️  air not installed, running without auto-reload..."; \
		go run ./cmd/platform; \
	fi

dev-tenant:
	@echo "💻 Starting tenant in development mode..."
	@echo "🔄 With auto-reload support (install air first: go install github.com/cosmtrek/air@latest)"
	@echo "📚 Swagger docs: http://localhost:8081/swagger/index.html"
	@echo "Press Ctrl+C to stop"
	@if command -v air > /dev/null 2>&1; then \
		air -c .air.toml; \
	else \
		@echo "⚠️  air not installed, running without auto-reload..."; \
		go run ./cmd/tenant; \
	fi

# ════════════════════════════════════════════════════════════════
# TESTS
# ════════════════════════════════════════════════════════════════
test: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/auth/tests ./modules/tenancy/tests ./modules/inventory/tests ./modules/sales/tests -timeout=120s

test-auth: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/auth/tests -timeout=120s

test-users: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/users/tests -timeout=120s

test-core: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/core/tests -timeout=120s

test-tenancy: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/tenancy/tests -timeout=120s

test-tracking: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/tracking/tests -timeout=120s

test-inventory: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/inventory/tests -timeout=120s

test-sales: db-reset
	go test -tags=integration $(GOFLAGS) ./modules/sales/tests -timeout=120s

test-all: db-reset
	@echo "🧪 Running all module tests..."
	go test -tags=integration $(GOFLAGS) ./modules/auth/tests -timeout=120s
	go test -tags=integration $(GOFLAGS) ./modules/users/tests -timeout=120s
	go test -tags=integration $(GOFLAGS) ./modules/tenancy/tests -timeout=120s
	go test -tags=integration $(GOFLAGS) ./modules/tracking/tests -timeout=120s
	go test -tags=integration $(GOFLAGS) ./modules/inventory/tests -timeout=120s
	go test -tags=integration $(GOFLAGS) ./modules/sales/tests -timeout=120s
	@echo "✅ All tests completed!"

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
	@echo "✅ Services started. Check with: docker-compose ps"

docker-down:
	@echo "🛑 Stopping Docker services..."
	docker-compose down
	@echo "✅ Services stopped"

# ════════════════════════════════════════════════════════════════
# CLEANUP
# ════════════════════════════════════════════════════════════════
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@echo "✅ Cleanup complete"

.DEFAULT_GOAL := help
