.PHONY: run build migrate seed clean tidy start fmt check check-go check-python check-shell check-docs test-integration dev-config dev-up dev-ready dev-down dev-reset dev-clean-data brew-minio-ready brew-reset-data backfill-index

COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
SHAREO_AI_PORT ?= 8000

# One-click start (check services + build + launch + open browser)
start:
	@bash start.sh

# Start the server (bare, no service checks)
run:
	go run cmd/server/main.go

# Build binary
build:
	go build -o bin/shareo cmd/server/main.go
	@echo "Built: bin/shareo"

# Run database migration
migrate:
	@echo "Running database migration..."
	bash scripts/migrate_schema.sh
	@echo "Migration complete."

# Generate test data (50 users with posts)
seed:
	@echo "Generating test data..."
	mysql -u root -p"${MYSQL_PASS}" shareo < migrations/002_seed.sql
	mysql -u root -p"${MYSQL_PASS}" shareo < migrations/003_triggers.sql
	@echo "Seed complete."

# Tidy Go modules
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/

# Reset database (drop and recreate)
reset-db:
	@echo "Dropping and recreating database..."
	mysql -u root -p"${MYSQL_PASS}" -e "DROP DATABASE IF EXISTS shareo;"
	bash scripts/migrate_schema.sh
	@echo "Database reset complete."

# Full setup: migrate + seed + run
setup: migrate seed
	@echo "Setup complete. Run 'make run' to start."

# Format check — fails if any Go file is not gofmt compliant
fmt:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "ERROR: unformatted Go files:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "All Go files are properly formatted."

# Go validation without external services
check-go: fmt
	go vet ./...
	go test -count=1 ./...
	@echo "All Go checks passed."

# Python validation from the committed uv lockfile
check-python:
	cd ai-service && uv sync --locked --group dev
	cd ai-service && uv run --frozen ruff check .
	cd ai-service && uv run --frozen ruff format --check .
	cd ai-service && uv run --frozen pytest -m "not integration"
	@echo "All Python checks passed."

# One-command validation: both runtimes, no external services
check: check-go check-python check-shell check-docs
	@echo "All checks passed."

check-shell:
	bash -n start.sh
	find scripts -type f -name '*.sh' -exec bash -n {} +
	find deploy -type f -name '*.sh' -exec bash -n {} +
	@echo "All shell scripts passed syntax validation."

check-docs:
	python3 scripts/check_docs.py

test-integration:
	@test -n "$$SHAREO_TEST_MYSQL_DSN" || (echo "SHAREO_TEST_MYSQL_DSN is required (database name must end in _test)" && exit 2)
	@test -n "$$SHAREO_TEST_REDIS_URL" || (echo "SHAREO_TEST_REDIS_URL is required" && exit 2)
	go test -count=1 -tags=integration ./...
	cd ai-service && uv run --frozen pytest -m integration

dev-config:
	$(COMPOSE) -f deploy/docker-compose.yml config --quiet

dev-up:
	$(COMPOSE) -f deploy/docker-compose.yml up -d --wait

dev-ready:
	curl --fail --silent http://localhost:$(SHAREO_AI_PORT)/healthz
	curl --fail --silent http://localhost:$(SHAREO_AI_PORT)/readyz

dev-down:
	$(COMPOSE) -f deploy/docker-compose.yml down

dev-reset:
	$(COMPOSE) -f deploy/docker-compose.yml down -v
	$(COMPOSE) -f deploy/docker-compose.yml up -d --wait

dev-clean-data:
	$(COMPOSE) -f deploy/docker-compose.yml down -v

brew-minio-ready:
	bash scripts/start_minio_homebrew.sh
	curl --noproxy '*' --fail --silent --show-error http://127.0.0.1:9000/minio/health/live

brew-reset-data:
	CONFIRM=YES bash scripts/reset_homebrew_data.sh

backfill-index: ## 将所有 approved 帖子重新投递到语义搜图索引队列
	go run ./cmd/backfill-index

# Show help
help:
	@echo "ShareO Makefile targets:"
	@echo "  make start     - 🚀 一键启动(检查服务+编译+打开浏览器)"
	@echo "  make run       - Start development server (bare)"
	@echo "  make build     - Build binary"
	@echo "  make check     - Run fmt check + vet + test"
	@echo "  make check-go  - Run Go format, vet, and unit tests"
	@echo "  make check-python - Run Python lint, format check, and unit tests"
	@echo "  make check-shell - Validate all shell scripts"
	@echo "  make check-docs - Validate local Markdown links"
	@echo "  make test-integration - Run tests that require MySQL/Redis"
	@echo "  make dev-up    - Start development dependencies with Docker Compose"
	@echo "  make dev-config - Validate Docker Compose configuration"
	@echo "  make dev-ready - Verify AI liveness and readiness"
	@echo "  make dev-down  - Stop development dependencies"
	@echo "  make dev-reset - Recreate development dependencies and their data"
	@echo "  make dev-clean-data - Stop services and remove development volumes"
	@echo "  make brew-minio-ready - Start/check Homebrew MinIO on ports 9000/9001"
	@echo "  make brew-reset-data - Destructively reset Homebrew ShareO data (CONFIRM=YES)"
	@echo "  make fmt       - Check code formatting (gofmt)"
	@echo "  make migrate   - Run database migration"
	@echo "  make seed      - Generate test data"
	@echo "  make setup     - migrate + seed"
	@echo "  make reset-db  - Drop & recreate database"
	@echo "  make tidy      - go mod tidy"
	@echo "  make clean     - Remove build artifacts"
