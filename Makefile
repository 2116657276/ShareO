.PHONY: run build migrate seed clean tidy start

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
	mysql -u root -p"${MYSQL_PASS}" < migrations/001_init.sql
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
	mysql -u root -p"${MYSQL_PASS}" < migrations/001_init.sql
	@echo "Database reset complete."

# Full setup: migrate + seed + run
setup: migrate seed
	@echo "Setup complete. Run 'make run' to start."

# Show help
help:
	@echo "ShareO Makefile targets:"
	@echo "  make start     - 🚀 一键启动(检查服务+编译+打开浏览器)"
	@echo "  make run       - Start development server (bare)"
	@echo "  make build     - Build binary"
	@echo "  make migrate   - Run database migration"
	@echo "  make seed      - Generate test data"
	@echo "  make setup     - migrate + seed"
	@echo "  make reset-db  - Drop & recreate database"
	@echo "  make tidy      - go mod tidy"
	@echo "  make clean     - Remove build artifacts"
