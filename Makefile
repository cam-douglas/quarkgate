.PHONY: deps infra-up infra-down migrate admin gateway worker test build all test-infra verify-mvp e2e-smoke e2e-live

deps:
	go mod tidy

infra-up:
	docker compose up -d

infra-down:
	docker compose down

migrate:
	go run ./cmd/admin migrate

admin:
	go run ./cmd/admin $(ARGS)

gateway:
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
