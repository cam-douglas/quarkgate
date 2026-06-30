#!/usr/bin/env bash
# Bootstrap repo Python venv + memory-stack dependencies (pip, CLIP, LanceDB, etc.)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VENV="${QUARK_PYTHON_VENV:-$ROOT/.venv}"

pick_python() {
  for candidate in \
    "${PYTHON:-}" \
    "/Library/Frameworks/Python.framework/Versions/3.12/bin/python3" \
    "$(command -v python3.12 2>/dev/null || true)" \
    "$(command -v python3 2>/dev/null || true)"; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  echo "No Python 3.12+ found. Install from python.org or: brew install python@3.12" >&2
  exit 1
}

PY="$(pick_python)"
echo "==> Python: $PY ($("$PY" --version))"

if [[ ! -d "$VENV" ]]; then
  echo "==> Creating venv at $VENV"
  "$PY" -m venv "$VENV"
fi

# shellcheck disable=SC1091
source "$VENV/bin/activate"

echo "==> Upgrading pip"
python -m pip install --upgrade pip setuptools wheel

echo "==> Core memory packages (editable)"
python -m pip install -q \
  -e "$ROOT/packages/vault-lib" \
  -e "$ROOT/packages/provider-adapters" \
  -e "$ROOT/packages/memory-lib" \
  -e "$ROOT/packages/memory-vector-lib[lance,clip]" \
  -e "$ROOT/packages/memory-graph-lib" \
  -e "$ROOT/packages/memory-bridge-lib" \
  -e "$ROOT/packages/policy-lib" \
  -e "$ROOT/services/memory-curator" \
  -e "$ROOT/services/memory-sleep-worker" \
  -e "$ROOT/services/letta-bridge" \
  -e "$ROOT/services/zep-ingest-worker" \
  -e "$ROOT/services/execution-controller" \
  -e "$ROOT/services/embed-worker" \
  -e "$ROOT/packages/rsi-lib" \
  -e "$ROOT/services/cost-sentinel" \
  -e "$ROOT/services/task-service" \
  -e "$ROOT/services/hermes" \
  -e "$ROOT/packages/skills-lib" \
  -e "$ROOT/packages/auth-lib" \
  -e "$ROOT/services/dev-dashboard" \
  -e "$ROOT/quarkgate/services/dev-unified" \
  2>/dev/null || python -m pip install -q \
  -e "$ROOT/packages/vault-lib" \
  -e "$ROOT/packages/provider-adapters" \
  -e "$ROOT/packages/memory-lib" \
  -e "$ROOT/packages/memory-vector-lib" \
  -e "$ROOT/packages/memory-graph-lib" \
  -e "$ROOT/packages/memory-bridge-lib" \
  -e "$ROOT/packages/policy-lib"

echo "==> LanceDB + CLIP runtime"
python -m pip install -q lancedb pyarrow transformers torch torchvision pillow httpx redis langsmith letta-client fastapi uvicorn

echo "==> pip: $(python -m pip --version)"
echo "==> Activate with: source $VENV/bin/activate"
echo "==> Or add to PATH: export PATH=\"$VENV/bin:\$PATH\""
