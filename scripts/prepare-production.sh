#!/usr/bin/env bash
# Pre-production validation — local checks before VPS deploy (Step 5).
set -eo pipefail
cd "$(dirname "$0")/.."
FAIL=0

check() {
  if "$@"; then
    echo "  OK   $1"
  else
    echo "  FAIL $1"
    FAIL=1
  fi
}

echo "==> Production readiness (local validation)"
check test -f .env.example
check grep -q 'VAULT_KEK' .env.example
check grep -q 'DATABASE_URL' .env.example
check grep -q 'EXECUTOR_BACKEND=e2b' .env.example
check test -f docs/runbooks/operator-runbook.md
check test -f ../docs/runbooks/spend-cap-policy.md
check test -f ../docs/runbooks/production-vps-bootstrap.md
check test -f ../infrastructure/e2b/quark-playwright/e2b.Dockerfile

if [ -f .env ]; then
  set +u
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
  if [ "${VAULT_KEK:-}" = "dev-only-32-byte-key-change-me!!" ]; then
    echo "  WARN VAULT_KEK still dev default — replace before prod (expected locally)"
  else
    echo "  OK   VAULT_KEK not dev placeholder"
  fi
fi

echo "==> Drill (dry-run)"
if [ -f ../scripts/production_readiness_drill.py ]; then
  python3 ../scripts/production_readiness_drill.py --dry-run >/tmp/quark-prod-drill.json 2>&1 && echo "  OK   production_readiness_drill --dry-run" || { echo "  FAIL production_readiness_drill"; FAIL=1; }
fi

echo "==> Gateway smoke"
if curl -sf "${DEV_GATEWAY_URL:-http://127.0.0.1:8090}/gateway/healthz" >/dev/null 2>&1; then
  bash scripts/test-dev-gateway.sh --strict && echo "  OK   test-gateway" || FAIL=1
else
  echo "  SKIP test-gateway (dev gateway not running)"
fi

if [ "$FAIL" -eq 0 ]; then
  echo ""
  echo "Local production prep PASS."
  echo "Owner still required: DO droplet, prod VAULT_KEK, Infisical, DNS/TLS."
else
  echo ""
  echo "Local production prep FAIL — fix items above."
  exit 1
fi
