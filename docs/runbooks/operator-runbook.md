# QuarkGate operator runbook (dev / staging / prod)

Quick reference for on-call operators. Production VPS bootstrap: [production-vps-bootstrap.md](../../../docs/runbooks/production-vps-bootstrap.md).

## Health checks

```bash
curl -sf http://127.0.0.1:8090/gateway/healthz    # unified dev
curl -sf http://127.0.0.1:8090/qg/healthz         # QuarkGate API
make test-gateway                                  # strict probe suite
```

## Ledger reconciliation

When cached Redis balance may drift from Postgres ledger:

```bash
cd quarkgate
source .env
USER_ID=$(psql "$DATABASE_URL" -t -c "SELECT id FROM users LIMIT 1;" | tr -d ' ')
go run ./cmd/admin reconcile-user "$USER_ID"
```

Expected: `no drift` or `fixed cached=… -> ledger=…` when `RECONCILE_AUTO_FIX=true`.

## DLQ replay (metering worker)

```bash
go run ./cmd/admin replay-dlq
```

Replay only after fixing root cause (Redis flush, worker crash). Inspect `$TMPDIR/quarkgate-dev/ledger-worker.log`.

## Load / latency sign-off

```bash
make load-reconciliation    # 1000 cheap metered requests + drift check
make streaming-p95          # proxy vs direct OpenRouter p95 delta
```

## Vault bootstrap (BYOK)

```bash
bash scripts/bootstrap-vault-from-env.sh
```

Never commit `.env`. Production: use Infisical per ADR-004.

## Emergency: stop dev stack

```bash
make dev-clean
```

## Production checklist (before traffic)

See [owner-operator-checklist.md](../owner-operator-checklist.md) Step 5 and `scripts/prepare-production.sh`.
