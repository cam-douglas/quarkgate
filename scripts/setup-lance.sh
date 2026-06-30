#!/usr/bin/env bash
# Initialize LanceDB tables for QuarkGate / quarkOS memory spectrum.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BRIDGE_VENV="$ROOT/services/kuzu-bridge/.venv"
QUARK_ROOT="$(cd "$ROOT/.." && pwd)"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

export LANCE_ENABLED="${LANCE_ENABLED:-true}"
export LANCE_RESET_TABLES="${LANCE_RESET_TABLES:-1}"

echo "==> LanceDB setup (LANCE_ENABLED=$LANCE_ENABLED, uri=${LANCEDB_URI:-data/lancedb})"

if [[ -d "$BRIDGE_VENV" ]]; then
  # shellcheck disable=SC1091
  source "$BRIDGE_VENV/bin/activate"
else
  python3 -m venv "$BRIDGE_VENV"
  source "$BRIDGE_VENV/bin/activate"
fi

pip install -q --upgrade pip
pip install -q lancedb pyarrow

export PYTHONPATH="$QUARK_ROOT/packages/memory-vector-lib:${PYTHONPATH:-}"
python3 << 'PY'
import os
from quark_memory_vector.lance_store import LanceStore

store = LanceStore()
reset = os.environ.get("LANCE_RESET_TABLES", "0") in {"1", "true", "yes"}
created = store.init_tables(reset=reset)
print(f"tables ready ({len(created)} initialized): {', '.join(created)}")
if store._use_real:
    print("list_tables:", store._table_names())
PY

echo "==> LanceDB setup complete"
