.PHONY: up reload-ai down reset logs doctor local-doctor local-infra-up local-infra-down dev-local start-local local-stop local-photo-seed test-local-stack check check-go check-python check-shell check-docs test-api test-integration test-integration-auto test-image-e2e test-ai-e2e test-agent-e2e test-agent-base test-agent-base-local test-degradation eval-ai eval-ai-machine eval-agent eval-agent-machine verify-agent-auto demo-seed backfill-index reconcile-index

COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
LOCAL_CACHE_ROOT ?= $(CURDIR)/.cache/shareo
GO_CACHE_DIR ?= $(LOCAL_CACHE_ROOT)/go-build
GO_MOD_CACHE ?= $(LOCAL_CACHE_ROOT)/go-mod
UV_CACHE_DIR ?= $(LOCAL_CACHE_ROOT)/uv
GO_ENV = GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE)
UV_ENV = UV_CACHE_DIR=$(UV_CACHE_DIR)

up:
	$(COMPOSE) up -d --build --wait

reload-ai:
	$(COMPOSE) build ai-service
	$(COMPOSE) up -d --force-recreate --no-deps --wait ai-service

down:
	$(COMPOSE) down --remove-orphans

reset:
	@test "$(CONFIRM)" = "YES" || (echo "Refusing reset: run make reset CONFIRM=YES" && exit 2)
	$(COMPOSE) down -v --remove-orphans
	$(COMPOSE) up -d --build --wait

logs:
	$(COMPOSE) logs -f --tail=200 app ai-service

doctor:
	bash scripts/dev_preflight.sh

local-doctor:
	bash scripts/local_runtime.sh doctor

local-infra-up:
	bash scripts/local_runtime.sh infra-up

local-infra-down:
	bash scripts/local_runtime.sh infra-down

dev-local:
	bash scripts/local_runtime.sh dev-local

start-local:
	bash scripts/start_local.sh

local-stop:
	bash scripts/local_runtime.sh local-stop

local-photo-seed:
	@test "$(CONFIRM)" = "YES" || (echo "Refusing local photo seed: run make local-photo-seed CONFIRM=YES" && exit 2)
	CONFIRM=YES python3 scripts/seed_local_photos.py

warm-ai:
	bash scripts/warm_ai.sh

check: check-go check-python check-shell check-docs
	@echo "All checks passed."

check-go:
	@files="$$(rg --files -g '*.go')"; if [ -n "$$files" ]; then bad="$$(gofmt -l $$files)"; if [ -n "$$bad" ]; then echo "$$bad"; exit 1; fi; fi
	$(GO_ENV) go vet ./...
	$(GO_ENV) go test -count=1 ./...

check-python:
	cd ai-service && $(UV_ENV) uv sync --locked --group dev
	cd ai-service && $(UV_ENV) uv run --frozen ruff check .
	cd ai-service && $(UV_ENV) uv run --frozen ruff format --check .
	cd ai-service && $(UV_ENV) uv run --frozen pytest -m "not integration"

check-shell:
	bash -n start.sh
	find scripts deploy -type f -name '*.sh' -exec bash -n {} +

check-docs:
	python3 scripts/check_docs.py

test-api:
	@test -x scripts/test_api.sh || (echo "scripts/test_api.sh is required" && exit 2)
	bash scripts/test_api.sh
	@test -x scripts/test_api_ai.sh || (echo "scripts/test_api_ai.sh is required" && exit 2)
	bash scripts/test_api_ai.sh

test-integration:
	@test -n "$$SHAREO_TEST_MYSQL_DSN" || (echo "SHAREO_TEST_MYSQL_DSN is required" && exit 2)
	@test -n "$$SHAREO_TEST_REDIS_URL" || (echo "SHAREO_TEST_REDIS_URL is required" && exit 2)
	$(GO_ENV) go test -count=1 -tags=integration ./...
	cd ai-service && $(UV_ENV) uv run --frozen pytest -m integration

test-integration-auto:
	bash scripts/test_integration_current.sh

test-image-e2e:
	bash scripts/test_image_search_e2e.sh

test-ai-e2e:
	@test -x scripts/test_ai_e2e.sh
	bash scripts/test_ai_e2e.sh

test-agent-e2e:
	@test -x scripts/test_agent_e2e.sh
	bash scripts/test_agent_e2e.sh

test-agent-base: check test-image-e2e test-ai-e2e test-agent-e2e
	@echo "Agent base automation passed."

test-local-stack:
	bash scripts/test_local_stack.sh

test-agent-base-local: check test-local-stack
	@echo "Native Agent base smoke checks passed."

test-degradation:
	@test -x scripts/test_degradation.sh
	bash scripts/test_degradation.sh

eval-ai:
	@test -f ai-service/app/commands/eval_ai.py || (echo "eval-ai is implemented in stage 7" && exit 2)
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.eval_ai \
		$(if $(BASE_URL),--base-url $(BASE_URL),) \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),) \
		$(if $(SCORING_OUTPUT),--scoring-output $(abspath $(SCORING_OUTPUT)),) \
		$(if $(SCORING_INPUT),--scoring-input $(abspath $(SCORING_INPUT)),) \
		$(if $(REPORT_INPUT),--report-input $(abspath $(REPORT_INPUT)),) \
		$(if $(filter 1,$(MACHINE_ONLY)),--machine-only,)

eval-ai-machine:
	$(MAKE) eval-ai MACHINE_ONLY=1 OUTPUT=$(if $(OUTPUT),$(OUTPUT),docs/eval/results/phase8a_rag_machine.json)

eval-agent:
	@test -f ai-service/app/commands/eval_agent.py || (echo "eval-agent is implemented in phase 8" && exit 2)
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.eval_agent \
		$(if $(BASE_URL),--base-url $(BASE_URL),) \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),) \
		$(if $(SCORING_OUTPUT),--scoring-output $(abspath $(SCORING_OUTPUT)),) \
		$(if $(SCORING_INPUT),--scoring-input $(abspath $(SCORING_INPUT)),) \
		$(if $(REPORT_INPUT),--report-input $(abspath $(REPORT_INPUT)),) \
		$(if $(filter 1,$(MACHINE_ONLY)),--machine-only,)

eval-agent-machine:
	$(MAKE) eval-agent MACHINE_ONLY=1 OUTPUT=$(if $(OUTPUT),$(OUTPUT),docs/eval/results/phase8a_agent_machine.json)

verify-agent-auto:
	bash scripts/verify_agent_auto.sh

demo-seed:
	@test -x scripts/demo_seed.sh || (echo "demo-seed is implemented in stage 7" && exit 2)
	bash scripts/demo_seed.sh

backfill-index:
	$(GO_ENV) go run ./cmd/backfill-index

reconcile-index:
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.reconcile_index $(if $(filter 1,$(APPLY)),--apply,)
