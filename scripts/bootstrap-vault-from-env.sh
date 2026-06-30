#!/usr/bin/env bash
# Bootstrap QuarkGate BYOK vault + dev user from quarkgate/.env (idempotent).
set -eo pipefail
cd "$(dirname "$0")/.."

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

ADMIN=(go run ./cmd/admin)
EMAIL="${QUARKGATE_DEV_EMAIL:-dev@quarkgate.dev}"
CREDITS="${QUARKGATE_DEPOSIT_CREDITS:-1000}"
KEY_LABEL="${QUARKGATE_KEY_LABEL:-dev-key}"

echo "==> Migrate ledger schema"
"${ADMIN[@]}" migrate

store_if_set() {
  local slug="$1" label="$2" val="$3"
  if [ -z "$val" ]; then
    echo "  skip $slug ($label unset)"
    return 0
  fi
  if "${ADMIN[@]}" store-credential "$slug" "$label" "$val" 2>/dev/null; then
    echo "  stored $slug / $label"
  else
    echo "  failed $slug / $label" >&2
    return 1
  fi
}

echo "==> Store vault credentials (BYOK downstream keys)"
store_if_set openrouter master_openrouter "${OPENROUTER_API_KEY:-}"
store_if_set apify master_apify "${APIFY_TOKEN:-}"
store_if_set letta master_letta "${LETTA_API_KEY:-}"
store_if_set supabase master_supabase "${SUPABASE_SERVICE_ROLE_KEY:-}"

echo "==> Patch provider base URLs from env"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<SQL
UPDATE provider_configs SET base_url = 'https://openrouter.ai/api/v1' WHERE provider_slug = 'openrouter';
UPDATE provider_configs SET base_url = 'https://api.apify.com/v2' WHERE provider_slug = 'apify';
UPDATE provider_configs SET base_url = '${LETTA_BASE_URL:-https://api.letta.com}' WHERE provider_slug = 'letta';
UPDATE provider_configs SET base_url = '${SUPABASE_URL:-https://example.supabase.co}' WHERE provider_slug = 'supabase';
SQL

echo "==> Ensure dev user + credits"
USER_LINE=$("${ADMIN[@]}" create-user "$EMAIL" 2>&1 || true)
if echo "$USER_LINE" | grep -q 'user_id='; then
  USER_ID=$(echo "$USER_LINE" | sed -n 's/.*user_id=\([^ ]*\).*/\1/p')
  echo "  created user $USER_ID"
else
  USER_ID=$(psql "$DATABASE_URL" -tAc "SELECT id FROM users WHERE email='$EMAIL' LIMIT 1")
  USER_ID=$(echo "$USER_ID" | tr -d '[:space:]')
  echo "  existing user $USER_ID"
fi

if [ -z "$USER_ID" ]; then
  echo "Could not resolve user_id" >&2
  exit 1
fi

"${ADMIN[@]}" deposit-credits "$USER_ID" "$CREDITS" >/dev/null
echo "  deposited $CREDITS credits"

KEY_COUNT=$(psql "$DATABASE_URL" -tAc "SELECT count(*) FROM quarkgate_keys WHERE user_id='$USER_ID' AND revoked_at IS NULL")
if [ "${KEY_COUNT:-0}" = "0" ]; then
  echo "==> Create API key (save output — shown once)"
  "${ADMIN[@]}" create-key "$USER_ID" "$KEY_LABEL"
else
  echo "==> Active API key exists for user (set QUARKGATE_KEY in .env if missing)"
  psql "$DATABASE_URL" -c "SELECT id, key_prefix, name, created_at FROM quarkgate_keys WHERE user_id='$USER_ID' AND revoked_at IS NULL;"
fi

echo "==> Driver health"
for p in openrouter apify letta supabase; do
  echo -n "  $p: "
  "${ADMIN[@]}" driver-health "$p" 2>&1 | head -c 120 || echo "FAIL"
  echo
done

echo "Done. Vault credentials are in Postgres (credential_vault_entries), encrypted with VAULT_KEK."
