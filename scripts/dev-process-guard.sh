#!/usr/bin/env bash
# Stop duplicate dev processes and optionally start the singleton guard watcher.
set -eo pipefail
cd "$(dirname "$0")/.."
GUARD="python3 scripts/dev-process-guard.py"

case "${1:-once}" in
  once)
    $GUARD --once --verbose
    ;;
  watch)
    exec $GUARD --supervise --interval "${GUARD_INTERVAL:-15}" --verbose
    ;;
  clean)
    echo "==> Stop unified dev gateway and legacy supervisors"
    pkill -f "quark_dev_unified.main" 2>/dev/null || true
    pkill -f "dev-process-guard.py --supervise" 2>/dev/null || true
    pkill -f "quark_dev_dashboard.main:app" 2>/dev/null || true
    rm -f "${TMPDIR:-/tmp}/quarkgate-dev/dev-unified.lock" 2>/dev/null || true
    sleep 1
    echo "==> Dedupe all guarded dev services"
    $GUARD --once --verbose
    echo "==> Done. Run 'make dev-up' to start a single unified gateway."
    ;;
  *)
    echo "Usage: $0 {once|watch|clean}"
    exit 1
    ;;
esac
