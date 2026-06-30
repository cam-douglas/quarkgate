.PHONY: deps infra-up infra-down migrate admin gateway worker test build all test-infra verify-mvp e2e-smoke e2e-live setup-kuzu setup-lance setup-memory sync-env kuzu-bridge test-kuzu setup-dev-services dev-dashboard dev-unified embed-worker dev dev-up memory-local guard guard-watch dev-clean resolve-infra test-gateway

deps:
	go mod tidy

setup-kuzu:
	bash scripts/setup-kuzu.sh

setup-python:
	bash scripts/bootstrap-python.sh

setup-lance:
	bash scripts/setup-lance.sh

setup-memory: setup-kuzu setup-lance sync-env
	@echo "Memory providers initialized (Kùzu + LanceDB)"

sync-env:
	python3 scripts/sync-memory-env.py

kuzu-bridge:
	@if [ -f services/kuzu-bridge/.venv/bin/activate ]; then \
		. services/kuzu-bridge/.venv/bin/activate; \
	else \
		echo "Run 'make setup-kuzu' first"; exit 1; \
	fi; \
	set -a && [ -f .env ] && . ./.env && set +a; \
	export KUZU_ENABLED=$${KUZU_ENABLED:-true}; \
	uvicorn quark_kuzu_bridge.main:app --app-dir services/kuzu-bridge --host 127.0.0.1 --port 8093 --reload

test-kuzu:
	cd services/kuzu-bridge && \
	if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
	. .venv/bin/activate && pip install -q -e ".[dev]" && \
	pytest -q tests

infra-up:
	docker compose up -d

infra-down:
	docker compose down

migrate:
	go run ./cmd/admin migrate

admin:
	go run ./cmd/admin $(ARGS)

gateway: guard
	go run ./cmd/gateway

worker:
	go run ./cmd/ledger-worker

test:
	go test ./...

build:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/ledger-worker ./cmd/ledger-worker
	go build -o bin/admin ./cmd/admin

all: deps build test

test-infra:
	@echo "Waiting for postgres and redis..."
	@sleep 2
	@redis-cli -p 6380 ping || (echo "redis down" && exit 1)
	go run ./cmd/admin migrate

verify-mvp:
	bash scripts/verify-mvp.sh

e2e-smoke:
	INTEGRATION=1 bash scripts/e2e-smoke.sh

e2e-live:
	go test -tags=e2e ./tests/e2e/... -count=1

setup-dev-services:
	@for svc in embed-worker dev-dashboard dev-unified; do \
		dir=services/$$svc; \
		if [ ! -d $$dir/.venv ]; then python3 -m venv $$dir/.venv; fi; \
		. $$dir/.venv/bin/activate; \
		if [ "$$svc" = "embed-worker" ]; then \
			pip install -q -e ../packages/provider-adapters -e $$dir; \
		elif [ "$$svc" = "dev-unified" ]; then \
			pip install -q -e services/dev-dashboard -e $$dir; \
		else \
			pip install -q -e $$dir; \
		fi; \
	done
	@echo "Dev service venvs ready"

embed-worker: setup-dev-services
	set -a && [ -f .env ] && . ./.env && set +a; \
	. services/embed-worker/.venv/bin/activate; \
	PYTHONPATH=../packages/provider-adapters:$$PYTHONPATH \
	uvicorn quark_embed_worker.main:app --app-dir services/embed-worker --host 127.0.0.1 --port $${EMBED_WORKER_PORT:-8087} --reload

dev-dashboard: setup-dev-services
	set -a && [ -f .env ] && . ./.env && set +a; \
	. services/dev-dashboard/.venv/bin/activate; \
	uvicorn quark_dev_dashboard.main:app --app-dir services/dev-dashboard --host 127.0.0.1 --port $${DEV_DASHBOARD_PORT:-8095} --reload

dev-unified: setup-dev-services
	set -a && [ -f .env ] && . ./.env && set +a; \
	export DEV_UNIFIED=1; \
	. services/dev-unified/.venv/bin/activate; \
	python -m quark_dev_unified.main

dev: dev-unified

dev-up:
	bash scripts/start-dev-stack.sh

resolve-infra:
	python3 scripts/resolve-dev-infra.py

bootstrap-vault:
	bash scripts/bootstrap-vault-from-env.sh

test-gateway:
	bash scripts/test-dev-gateway.sh --strict

load-reconciliation:
	set -a && [ -f .env ] && . ./.env && set +a; node scripts/load-reconciliation-test.js

streaming-p95:
	set -a && [ -f .env ] && . ./.env && set +a; node scripts/streaming-p95-benchmark.js

prepare-production:
	bash scripts/prepare-production.sh

build-e2b-playwright:
	bash scripts/build-e2b-playwright-template.sh


memory-local:
	bash scripts/start-memory-local.sh

guard:
	python3 scripts/dev-process-guard.py --once --verbose

guard-watch:
	python3 scripts/dev-process-guard.py --supervise --interval 15 --verbose

dev-clean:
	bash scripts/dev-process-guard.sh clean
