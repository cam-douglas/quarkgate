#!/usr/bin/env bash
# Build and register quark-playwright E2B template (Playwright + Chromium).
set -eo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEMPLATE_DIR="$ROOT/../infrastructure/e2b/quark-playwright"
cd "$ROOT"

if [ -f .env ]; then
  set +u
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

if [ -z "${E2B_API_KEY:-}" ]; then
  echo "E2B_API_KEY required in quarkgate/.env"
  exit 1
fi

export E2B_API_KEY

echo "==> Install E2B CLI (npx, no global required)"
CLI="npx --yes @e2b/cli@latest"

echo "==> Create template (E2B CLI v2)"
cd "$TEMPLATE_DIR"
$CLI template create quark-playwright -d e2b.Dockerfile --cpu-count 2 --memory-mb 2048 2>&1 | tee /tmp/e2b-quark-playwright-build.log

TEMPLATE_ID="quark-playwright"

if [ -z "$TEMPLATE_ID" ]; then
  echo "WARN: Could not parse template id from e2b template list — check /tmp/e2b-quark-playwright-build.log"
  echo "Set E2B_TEMPLATE_ID manually in .env after build completes."
  exit 0
fi

echo "==> Template id: $TEMPLATE_ID"
ENV_FILE="$ROOT/.env"
if grep -q '^E2B_TEMPLATE_ID=' "$ENV_FILE" 2>/dev/null; then
  sed -i.bak "s|^E2B_TEMPLATE_ID=.*|E2B_TEMPLATE_ID=$TEMPLATE_ID|" "$ENV_FILE"
else
  echo "E2B_TEMPLATE_ID=$TEMPLATE_ID" >> "$ENV_FILE"
fi

OS_ENV="$ROOT/../.env"
if [ -f "$OS_ENV" ]; then
  if grep -q '^E2B_TEMPLATE_ID=' "$OS_ENV"; then
    sed -i.bak "s|^E2B_TEMPLATE_ID=.*|E2B_TEMPLATE_ID=$TEMPLATE_ID|" "$OS_ENV"
  else
    echo "E2B_TEMPLATE_ID=$TEMPLATE_ID" >> "$OS_ENV"
  fi
fi

echo "==> Smoke: list sandboxes"
curl -sf -H "X-API-Key: $E2B_API_KEY" "${E2B_API_BASE:-https://api.e2b.dev}/sandboxes" >/dev/null && echo "  e2b API ok"

echo "Done. Set BROWSER_ACTORS_ENABLED=true and restart dev stack."
