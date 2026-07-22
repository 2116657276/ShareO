.PHONY: up down reset logs check check-go check-python check-shell check-docs test-integration test-image-e2e test-ai-e2e eval-ai demo-seed backfill-index reconcile-index

COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

up:
	$(COMPOSE) up -d --build --wait

down:
	$(COMPOSE) down --remove-orphans

reset:
	@test "$(CONFIRM)" = "YES" || (echo "Refusing reset: run make reset CONFIRM=YES" && exit 2)
	$(COMPOSE) down -v --remove-orphans
	$(COMPOSE) up -d --build --wait

logs:
	$(COMPOSE) logs -f --tail=200 app ai-service

check: check-go check-python check-shell check-docs
	@echo "All checks passed."

check-go:
	@if [ -n "$$(gofmt -l .)" ]; then gofmt -l .; exit 1; fi
	go vet ./...
	go test -count=1 ./...

check-python:
	cd ai-service && uv sync --locked --group dev
	cd ai-service && uv run --frozen ruff check .
	cd ai-service && uv run --frozen ruff format --check .
	cd ai-service && uv run --frozen pytest -m "not integration"

check-shell:
	bash -n start.sh
	find scripts deploy -type f -name '*.sh' -exec bash -n {} +

check-docs:
	python3 scripts/check_docs.py

test-integration:
	@test -n "$$SHAREO_TEST_MYSQL_DSN" || (echo "SHAREO_TEST_MYSQL_DSN is required" && exit 2)
	@test -n "$$SHAREO_TEST_REDIS_URL" || (echo "SHAREO_TEST_REDIS_URL is required" && exit 2)
	go test -count=1 -tags=integration ./...
	cd ai-service && uv run --frozen pytest -m integration

test-image-e2e:
	bash scripts/test_image_search_e2e.sh

test-ai-e2e:
	@test -x scripts/test_ai_e2e.sh || (echo "test-ai-e2e is implemented in stage 7" && exit 2)
	bash scripts/test_ai_e2e.sh

eval-ai:
	@test -f ai-service/app/commands/eval_ai.py || (echo "eval-ai is implemented in stage 7" && exit 2)
	cd ai-service && uv run --frozen python -m app.commands.eval_ai

demo-seed:
	@test -x scripts/demo_seed.sh || (echo "demo-seed is implemented in stage 7" && exit 2)
	bash scripts/demo_seed.sh

backfill-index:
	go run ./cmd/backfill-index

reconcile-index:
	cd ai-service && uv run --frozen python -m app.commands.reconcile_index $(if $(filter 1,$(APPLY)),--apply,)
