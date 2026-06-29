#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== verify-mvp =="
go test ./...
npm run validate:manifests
npm run test:drivers
echo "== test-infra =="
make test-infra
echo "verify-mvp OK"
