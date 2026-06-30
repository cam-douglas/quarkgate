#!/usr/bin/env bash
# Start quarkOS memory-spectrum services locally (no Docker build required).
# Memory host: Anytype API + Memory Curator co-located on this machine.
set -eo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

if [ -f .env ]; then set -a; source .env; set +a; fi
if [ -f quarkgate/.env ]; then set -a; source quarkgate/.env; set +a; fi

export REDIS_URL="${REDIS_URL:-redis://127.0.0.1:6380}"
export MEMORY_MATRIX_PATH="${MEMORY_MATRIX_PATH:-config/memory_matrix.yaml}"
export MEMORY_SPECTRUM_CONFIG_PATH="${MEMORY_SPECTRUM_CONFIG_PATH:-config/memory_spectrum.yaml}"

LOG="${TMPDIR:-/tmp}/quark-memory-local"
mkdir -p "$LOG"
ANYTYPE_BIN="${ANYTYPE_BIN:-$HOME/.local/bin/anytype}"
ANYTYPE_PORT="${ANYTYPE_PORT:-31012}"
if [[ "${ANYTYPE_API_URL:-}" =~ :([0-9]+)$ ]]; then
  ANYTYPE_PORT="${BASH_REMATCH[1]}"
fi

start() {
  local name="$1" port="$2" cmd="$3"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "  :$port already listening ($name)"
    return
  fi
  nohup bash -c "$cmd" >"$LOG/$name.log" 2>&1 &
  echo "  $name -> http://127.0.0.1:$port (log: $LOG/$name.log)"
}

echo "==> Memory host (Anytype + Memory Curator on same server)"
if [ "${ANYTYPE_ENABLED:-false}" = "true" ]; then
  if [ -x "$ANYTYPE_BIN" ] || command -v anytype >/dev/null 2>&1; then
    start anytype "$ANYTYPE_PORT" "${ANYTYPE_BIN:-anytype} serve --listen-address 127.0.0.1:${ANYTYPE_PORT} -q"
  else
    echo "  anytype CLI not found — set ANYTYPE_BIN or install anytype-cli"
  fi
else
  echo "  ANYTYPE_ENABLED=false — skipping anytype serve"
fi

echo "==> Installing memory packages (editable)"
python3 -m pip install -q \
  -e packages/vault-lib \
  -e packages/provider-adapters \
  -e packages/memory-lib \
  -e packages/memory-vector-lib \
  -e packages/memory-graph-lib \
  -e packages/memory-bridge-lib \
  -e packages/policy-lib \
  -e services/memory-curator \
  -e services/memory-sleep-worker \
  2>/dev/null || true

start curator 8082 "cd $ROOT/services/memory-curator && python3 run.py"

echo "==> Sleep Worker :8088"
start sleep 8088 "python3 -m quark_memory_sleep_worker.main"

echo "==> Letta Bridge :8089"
start letta 8089 "cd $ROOT && python3 -m uvicorn quark_letta_bridge.main:app --app-dir services/letta-bridge --host 127.0.0.1 --port 8089"

echo "==> Zep Ingest :8091"
python3 -m pip install -q redis 2>/dev/null || true
start zep 8091 "cd $ROOT && PYTHONPATH=$ROOT/packages/memory-bridge-lib:$ROOT/packages/policy-lib python3 -m uvicorn quark_zep_ingest.consumer:app --app-dir services/zep-ingest-worker --host 127.0.0.1 --port 8091"

echo "==> Execution Controller :8083 (e2b sandboxes — no Docker)"
python3 -m pip install -q -e services/execution-controller -e packages/auth-lib 2>/dev/null || true
start execution 8083 "cd $ROOT && EXECUTOR_BACKEND=e2b DOCKER_ENABLED=false EXECUTION_POLICY_PATH=config/execution_policy.yaml python3 -m uvicorn quark_execution_controller.main:app --app-dir services/execution-controller --host 127.0.0.1 --port 8083"

echo "==> Task service (:8081) + Hermes (:8084)"
python3 -m pip install -q \
  -e packages/rsi-lib \
  -e services/cost-sentinel \
  -e services/task-service \
  -e services/hermes \
  -e packages/skills-lib \
  -e packages/vault-lib \
  -e packages/auth-lib \
  2>/dev/null || true
start task 8081 "cd $ROOT/services/task-service && python3 run.py"
start hermes 8084 "cd $ROOT/services/hermes && python3 run.py"

sleep 4
for url in \
  "http://127.0.0.1:${ANYTYPE_PORT}/v1/spaces" \
  http://127.0.0.1:8082/health \
  http://127.0.0.1:8083/health \
  http://127.0.0.1:8084/health \
  http://127.0.0.1:8088/health \
  http://127.0.0.1:8089/healthz \
  http://127.0.0.1:8091/health \
  http://127.0.0.1:8081/health; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "$url" || echo "000")
  echo "  $url -> $code"
done
