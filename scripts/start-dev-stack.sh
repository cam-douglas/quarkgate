#!/usr/bin/env bash
# Start QuarkGate dev stack without Docker — unified gateway + native/cloud infra.
set -eo pipefail
cd "$(dirname "$0")/.."
REPO_ROOT="$(cd .. && pwd)"
LOG_DIR="${TMPDIR:-/tmp}/quarkgate-dev"
mkdir -p "$LOG_DIR"

if [ -f .env ]; then
  set +u
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

echo "==> Resolve infra (host Redis, Supabase Postgres, e2b execution — no Docker)"
python3 scripts/resolve-dev-infra.py
set +u
set -a && source ./.env && set +a

export DEV_UNIFIED=1
export DOCKER_ENABLED=false
export PATH="${REPO_ROOT}/.venv/bin:${PATH}"

echo "==> Clean legacy dev processes"
python3 - <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, str(Path("scripts").resolve()))
import dev_supervisor as s
s.load_env()
s.kill_legacy_dev_processes()
s.run_once(verbose=True)
PY

echo "==> Waiting for Redis at ${REDIS_URL}"
for _ in $(seq 1 15); do
  redis-cli -u "${REDIS_URL}" ping >/dev/null 2>&1 && break
  sleep 1
done

echo "==> Migrations (Supabase or local Postgres)"
go run ./cmd/admin migrate || true

make setup-dev-services >/dev/null 2>&1 || make setup-dev-services

UNIFIED_VENV="services/dev-unified/.venv/bin/python3"
if [ ! -x "$UNIFIED_VENV" ]; then
  echo "dev-unified venv missing — run make setup-dev-services"
  exit 1
fi

if lsof -nP -iTCP:"${DEV_GATEWAY_PORT:-8090}" -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo "Dev gateway already listening on :${DEV_GATEWAY_PORT:-8090}"
else
  echo "==> Unified dev gateway (:${DEV_GATEWAY_PORT:-8090})"
  python3 - <<PY
import os
import subprocess
import sys
from pathlib import Path

root = Path("${REPO_ROOT}/quarkgate")
venv = root / "services/dev-unified/.venv/bin/python3"
log = Path("${LOG_DIR}/dev-unified.log")
env = os.environ.copy()
env.update(
    {
        "DEV_UNIFIED": "1",
        "DEV_GATEWAY_PORT": "${DEV_GATEWAY_PORT:-8090}",
        "DEV_GATEWAY_URL": "${DEV_GATEWAY_URL:-http://127.0.0.1:8090}",
        "PATH": "${REPO_ROOT}/.venv/bin:" + env.get("PATH", ""),
    }
)
log.parent.mkdir(parents=True, exist_ok=True)
with open(log, "a", encoding="utf-8") as fh:
    proc = subprocess.Popen(
        [str(venv), "-m", "quark_dev_unified.main"],
        cwd=str(root),
        env=env,
        stdout=fh,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
print(proc.pid)
PY
  echo "    dev-unified -> $LOG_DIR/dev-unified.log"
  for _ in $(seq 1 30); do
    curl -sf "${DEV_GATEWAY_URL:-http://127.0.0.1:8090}/gateway/healthz" >/dev/null 2>&1 && break
    sleep 1
  done
fi

echo ""
echo "Dev gateway:   ${DEV_GATEWAY_URL}/"
echo "  Dashboard:   ${DEV_GATEWAY_URL}/"
echo "  QuarkGate:   ${QUARKGATE_URL}/healthz"
echo "  Execution:   e2b (DOCKER_ENABLED=false)"
echo "Logs:          $LOG_DIR"
echo "Stop:          make dev-clean"

if command -v open >/dev/null 2>&1; then
  open "${DEV_GATEWAY_URL}/"
fi
