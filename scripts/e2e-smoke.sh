#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

make infra-up
sleep 3
make migrate
go test -tags=integration ./internal/metering/... -count=1
echo "e2e-smoke OK"
