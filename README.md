# QuarkGate

Unified reverse proxy, authentication wrapper, and micro-billing gateway for agentic AI infrastructure.

**Interim product goal (ADR-036):** **BYOK** — tenants use their own provider keys; POC = sole operator until provider partnerships enable resale.

One API key. Metering and routing. Provider credentials are **bring-your-own-key**, not operator master-key resale (deferred).

## Quick start

```bash
make infra-up
make deps
make migrate

# Create user, deposit credits, create key
go run ./cmd/admin create-user dev@quarkgate.dev
go run ./cmd/admin deposit-credits <user_id> 1000
go run ./cmd/admin create-key <user_id> dev-key
go run ./cmd/admin revoke-key <key_id>

# Store provider credentials (master keys)
go run ./cmd/admin store-credential openrouter master_openrouter <openrouter-api-key>

# Run gateway + ledger worker
make gateway
make worker
```

## Architecture

- **Go edge** (`cmd/gateway`) — auth, solvency holds, streaming proxy, metering events
- **Ledger worker** (`cmd/ledger-worker`) — async capture/release, hold sweeper, reconciliation
- **TypeScript drivers** (`drivers/`) — pluggable provider transforms via JSON IPC
- **PostgreSQL** — ACID ledger, usage logs, credential vault
- **Redis** — balance cache, holds, rate limits, metering stream

## Credit unit

1 QuarkGate Credit = 1,000,000 micro_credits. Default: 1 Credit ≈ $0.01 USD (`CREDIT_USD_MICRO=10000`).

## API

- `POST /v1/chat/completions` — OpenAI-compat → OpenRouter
- `POST /v1/quarkgate` — QuarkGate envelope for any provider
- `POST /v1/providers/{provider}{path}` — provider-specific routes

Bearer: `Authorization: Bearer qg_live_...`

## Environment

| Variable | Default |
|----------|---------|
| `LISTEN_ADDR` | `:8080` |
| `DATABASE_URL` | `postgres://quarkgate:quarkgate@localhost:5433/quarkgate?sslmode=disable` |
| `REDIS_URL` | `redis://localhost:6380/0` |
| `VAULT_KEK` | dev-only — change in production |
| `DRIVERS_PATH` | `drivers` |
| `CREDIT_USD_MICRO` | `10000` |
| `STREAM_IDLE_SEC` | `30` |
| `STREAM_MAX_SEC` | `1800` |

Copy `.env.example` to `.env` for BYOK POC credentials. See `docs/adr/036-byok-credential-model.md`.

Memory providers (WP11) mirror root `quarkOS/.env` — including `KUZU_*`, `LANCEDB_*`, `ZEP_*`, etc.

## Memory — Kùzu graph bridge

```bash
make setup-kuzu      # clone already at tools/repos/kuzu; init embedded DB
make kuzu-bridge     # :8093 when KUZU_ENABLED=true
```

See `services/kuzu-bridge/README.md` and `tools/repos/README.md`.

## Drivers

Copy `drivers/_template/` to add a provider. See `drivers/_template/DRIVER.md`.

## Docs

- Architecture plans: `docs/plans/phase_1_foundation.plan.md`, `docs/plans/phase_2_proxy_auth.plan.md`, `docs/plans/phase_3_metering.plan.md`
- JSON schemas: `schemas/`

## License

MIT
