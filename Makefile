.DEFAULT_GOAL := up

.PHONY: start up stop down doctor logs start-local dev-local local-doctor \
	local-infra-up local-infra-down local-stop local-logs local-photo-seed \
	prepare-search-eval prepare-current-ai-eval warm-ai \
	compose-up compose-down compose-reset compose-logs compose-reload-ai \
	test-local-stack check check-go check-python check-shell check-docs test-api \
	test-integration test-integration-auto \
	eval-ai eval-ai-machine eval-agent eval-agent-machine eval-post-search \
	eval-image-search-local eval-ai-judge eval-dataset-audit \
	worktree-manifest test-evidence-tools test-browser-smoke resume-audit final-evidence \
	backfill-index reconcile-index bootstrap-postgres

# Local host services are the only supported runtime. PostgreSQL 17, Redis and
# MinIO run as host services; no container runtime is consulted.
SHAREO_RUNTIME ?= local
LOCAL_CACHE_ROOT ?= $(CURDIR)/.cache/shareo
GO_CACHE_DIR ?= $(LOCAL_CACHE_ROOT)/go-build
GO_MOD_CACHE ?= $(LOCAL_CACHE_ROOT)/go-mod
UV_CACHE_DIR ?= $(LOCAL_CACHE_ROOT)/uv
GO_ENV = GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE)
UV_ENV = UV_CACHE_DIR=$(UV_CACHE_DIR)

start up start-local:
	SHAREO_RUNTIME=local bash scripts/start_local.sh

stop down local-stop:
	SHAREO_RUNTIME=local bash scripts/local_runtime.sh local-stop

doctor local-doctor:
	SHAREO_RUNTIME=local bash scripts/local_runtime.sh doctor

logs local-logs:
	@state_dir="$${SHAREO_LOCAL_STATE_DIR:-/tmp/shareo-local}"; \
		echo "Go log: $$state_dir/app.log"; \
		echo "AI log: $$state_dir/ai.log"; \
		tail -n 120 "$$state_dir/app.log" "$$state_dir/ai.log" 2>/dev/null || true

local-infra-up:
	SHAREO_RUNTIME=local bash scripts/local_runtime.sh infra-up

bootstrap-postgres:
	bash scripts/bootstrap_postgres.sh

local-infra-down:
	SHAREO_RUNTIME=local bash scripts/local_runtime.sh infra-down

dev-local:
	SHAREO_RUNTIME=local bash scripts/local_runtime.sh dev-local

local-photo-seed:
	@test "$(CONFIRM)" = "YES" || (echo "Refusing local photo seed: run make local-photo-seed CONFIRM=YES" && exit 2)
	CONFIRM=YES python3 scripts/seed_local_photos.py

warm-ai:
	bash scripts/warm_ai.sh

prepare-search-eval:
	python3 scripts/prepare_local_search_eval.py
	python3 scripts/prepare_local_image_eval.py

prepare-current-ai-eval:
	python3 scripts/prepare_current_ai_eval.py --version $${VERSION:-v2}

compose-up compose-reload-ai compose-down compose-reset compose-logs:
	@echo "[FAIL] Compose runtime is retired; use the native PostgreSQL/Redis/MinIO runtime" >&2
	@exit 2

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
	bash scripts/test_api.sh
	bash scripts/test_api_ai.sh

test-integration:
	@test -n "$$SHAREO_TEST_POSTGRES_DSN" || (echo "SHAREO_TEST_POSTGRES_DSN is required" && exit 2)
	@test -n "$$SHAREO_TEST_AI_DATABASE_URL" || (echo "SHAREO_TEST_AI_DATABASE_URL is required" && exit 2)
	@test -n "$$SHAREO_TEST_REDIS_URL" || (echo "SHAREO_TEST_REDIS_URL is required" && exit 2)
	$(GO_ENV) go test -count=1 -tags=integration ./...
	cd ai-service && $(UV_ENV) uv run --frozen pytest -m integration

test-integration-auto:
	SHAREO_RUNTIME=local bash scripts/test_integration_current.sh

test-local-stack:
	bash scripts/test_local_stack.sh

eval-ai:
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.eval_ai \
		$(if $(BASE_URL),--base-url $(BASE_URL),) \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),) \
		$(if $(SCORING_OUTPUT),--scoring-output $(abspath $(SCORING_OUTPUT)),) \
		$(if $(SCORING_INPUT),--scoring-input $(abspath $(SCORING_INPUT)),) \
		$(if $(REPORT_INPUT),--report-input $(abspath $(REPORT_INPUT)),) \
		$(if $(filter 1,$(MACHINE_ONLY)),--machine-only,)

eval-ai-machine:
	$(MAKE) eval-ai MACHINE_ONLY=1 OUTPUT=$(if $(OUTPUT),$(OUTPUT),.local/shareo/eval/rag-image.json)

eval-agent:
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.eval_agent \
		$(if $(BASE_URL),--base-url $(BASE_URL),) \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),) \
		$(if $(SCORING_OUTPUT),--scoring-output $(abspath $(SCORING_OUTPUT)),) \
		$(if $(SCORING_INPUT),--scoring-input $(abspath $(SCORING_INPUT)),) \
		$(if $(REPORT_INPUT),--report-input $(abspath $(REPORT_INPUT)),) \
		$(if $(filter 1,$(MACHINE_ONLY)),--machine-only,)

eval-agent-machine:
	$(MAKE) eval-agent MACHINE_ONLY=1 OUTPUT=$(if $(OUTPUT),$(OUTPUT),.local/shareo/eval/agent.json)

eval-ai-judge:
	@test -n "$(RAG_REPORT)" || (echo "RAG_REPORT is required" && exit 2)
	@test -n "$(AGENT_REPORT)" || (echo "AGENT_REPORT is required" && exit 2)
	@test -n "$(OUTPUT)" || (echo "OUTPUT is required" && exit 2)
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.ai_judge \
		--rag-report $(abspath $(RAG_REPORT)) \
		--agent-report $(abspath $(AGENT_REPORT)) \
		--output $(abspath $(OUTPUT))

eval-dataset-audit:
	@test -n "$(RAG_DATASET)" || (echo "RAG_DATASET is required" && exit 2)
	@test -n "$(AGENT_DATASET)" || (echo "AGENT_DATASET is required" && exit 2)
	@test -n "$(OUTPUT)" || (echo "OUTPUT is required" && exit 2)
	@test -n "$(MANIFEST)" || (echo "MANIFEST is required" && exit 2)
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.ai_judge \
		--rag-dataset $(abspath $(RAG_DATASET)) \
		--agent-dataset $(abspath $(AGENT_DATASET)) \
		--manifest $(abspath $(MANIFEST)) \
		--dataset-support-output $(abspath $(OUTPUT))

worktree-manifest:
	python3 scripts/collect_worktree_manifest.py \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),) \
		$(if $(filter 1,$(SOURCE_ONLY)),--source-only,)

test-evidence-tools:
	python3 -m unittest scripts/test_final_evidence.py

test-browser-smoke:
	bash scripts/test_browser_smoke.sh

resume-audit:
	python3 scripts/collect_final_evidence.py \
		$(if $(RUN_ID),--run-id $(RUN_ID),) \
		--ai-eval-runs $${AI_EVAL_RUNS:-3} \
		$(if $(filter 1,$(BROWSER_SMOKE)),--browser-smoke,)

eval-post-search:
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.eval_post_search \
		$(if $(BASE_URL),--base-url $(BASE_URL),) \
		$(if $(AI_URL),--ai-url $(AI_URL),) \
		$(if $(DATASET),--dataset $(abspath $(DATASET)),) \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),)

eval-image-search-local:
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.eval_image_search \
		--dataset $(abspath $(if $(DATASET),$(DATASET),.local/shareo/eval/image_search_local_v1.jsonl)) \
		$(if $(BASE_URL),--base-url $(BASE_URL),) \
		$(if $(OUTPUT),--output $(abspath $(OUTPUT)),) \
		$(if $(SCORE_THRESHOLD),--score-threshold $(SCORE_THRESHOLD),)

final-evidence:
	python3 scripts/collect_final_evidence.py

backfill-index:
	$(GO_ENV) go run ./cmd/backfill-index

reconcile-index:
	cd ai-service && $(UV_ENV) uv run --frozen python -m app.commands.reconcile_index $(if $(filter 1,$(APPLY)),--apply,)
