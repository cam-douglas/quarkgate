#!/usr/bin/env bash
# Initialize Kùzu for QuarkGate: Python deps, optional fork build, schema + demo seed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KUZU_REPO="${KUZU_REPO_PATH:-$ROOT/tools/repos/kuzu}"
BRIDGE_DIR="$ROOT/services/kuzu-bridge"
VENV="$BRIDGE_DIR/.venv"
DB_PATH="${KUZU_DB_PATH:-$ROOT/data/kuzu/memory_graph.kuzu}"

export KUZU_DB_PATH="$DB_PATH"

echo "==> QuarkGate Kùzu setup"
echo "    repo:  $KUZU_REPO"
echo "    db:    $DB_PATH"
echo "    venv:  $VENV"

if [[ ! -d "$KUZU_REPO/.git" ]]; then
  echo "ERROR: Kùzu fork not found at $KUZU_REPO — run: git clone https://github.com/cam-douglas/kuzu.git $KUZU_REPO"
  exit 1
fi

python3 -m venv "$VENV"
# shellcheck disable=SC1091
source "$VENV/bin/activate"
pip install -q --upgrade pip
pip install -q -e "$BRIDGE_DIR"

if [[ "${KUZU_BUILD_FROM_SOURCE:-0}" == "1" ]]; then
  echo "==> Building Kùzu Python bindings from fork (this may take several minutes)..."
  make -C "$KUZU_REPO" python
  cp "$KUZU_REPO"/tools/python_api/src_py/*.py "$KUZU_REPO"/tools/python_api/build/kuzu/ 2>/dev/null || true
  export PYTHONPATH="$KUZU_REPO/tools/python_api/build:${PYTHONPATH:-}"
  python3 -c "import kuzu; print('kuzu from source build OK')"
else
  echo "==> Using PyPI kuzu wheel (set KUZU_BUILD_FROM_SOURCE=1 to compile the fork)"
  python3 -c "import kuzu; print('kuzu', kuzu.__version__, 'OK')"
fi

mkdir -p "$(dirname "$DB_PATH")"
python3 "$ROOT/scripts/init-kuzu-db.py" --seed-demo

echo "==> Done. Start bridge with: make kuzu-bridge (from quarkgate/)"
