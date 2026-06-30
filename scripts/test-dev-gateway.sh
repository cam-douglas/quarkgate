#!/usr/bin/env bash
# Test QuarkGate unified dev gateway and child services.
# Usage: bash scripts/test-dev-gateway.sh [--json] [--strict]
set -eo pipefail
cd "$(dirname "$0")/.."

JSON=0
STRICT=0
for arg in "$@"; do
  case "$arg" in
    --json) JSON=1 ;;
    --strict) STRICT=1 ;;
  esac
done

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

BASE="${DEV_GATEWAY_URL:-http://127.0.0.1:8090}"
FAIL=0

check() {
  local name="$1" url="$2" expect="${3:-200}"
  local code
  code=$(curl -sf -o /dev/null -w "%{http_code}" --max-time 15 "$url" 2>/dev/null || echo "000")
  if [ "$code" = "$expect" ] || { [ "$expect" = "2xx" ] && [ "${code:0:1}" = "2" ]; }; then
    echo "  OK   $name ($code) $url"
  else
    echo "  FAIL $name ($code) $url"
    FAIL=$((FAIL + 1))
  fi
}

echo "==> Unified gateway: $BASE"
check "gateway-health" "$BASE/gateway/healthz" 200
check "dashboard" "$BASE/" 200

if curl -sf --max-time 120 -X POST "$BASE/api/test-all" -H "Accept: application/json" -o /tmp/quarkgate-test-all.json; then
  if [ "$JSON" = "1" ]; then
    cat /tmp/quarkgate-test-all.json
    echo
  fi
  read -r UP DOWN ISSUE SKIPPED < <(
    python3 - <<'PY'
import json
d = json.load(open("/tmp/quarkgate-test-all.json"))
s = d["summary"]
print(s.get("up", 0), s.get("down", 0), s.get("issue", 0), s.get("skipped", 0))
PY
  )
  echo "  API test summary: up=$UP down=$DOWN issue=$ISSUE skipped=$SKIPPED"
  if [ "$STRICT" = "1" ] && [ "${DOWN:-0}" != "0" ]; then
    FAIL=$((FAIL + DOWN))
  fi
  if [ "$STRICT" = "1" ] && [ "${ISSUE:-0}" != "0" ]; then
    FAIL=$((FAIL + ISSUE))
  fi
else
  echo "  FAIL api/test-all (connection or timeout)"
  FAIL=$((FAIL + 1))
fi

echo "==> Proxy smoke"
check "quarkgate-proxy" "$BASE/qg/healthz" 200
check "task-proxy" "$BASE/tasks/health" 2xx
check "memory-proxy" "$BASE/memory/health" 2xx
check "exec-proxy" "$BASE/exec/health" 2xx
check "hermes-proxy" "$BASE/hermes/health" 2xx

if [ "$FAIL" -gt 0 ]; then
  echo "==> $FAIL check(s) failed"
  exit 1
fi
echo "==> All checks passed"
