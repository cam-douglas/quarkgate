# Swarm minimal orchestrator

End-to-end smoke across OpenRouter, Apify, Letta, and Supabase using one QuarkGate API key.

## Prerequisites

- Gateway running (`make gateway`)
- Ledger worker running (`make worker`)
- User with credits and API key

```bash
make infra-up
make migrate
go run ./cmd/admin create-user you@example.com
go run ./cmd/admin deposit-credits <user_id> 1000
go run ./cmd/admin create-key <user_id> smoke
```

## Smoke (status checks)

```bash
export QUARKGATE_URL=http://localhost:8080
export QUARKGATE_KEY=qg_live_...
node examples/swarm-minimal/orchestrator.js
```

## Live E2E (real providers)

Create `.env.e2e` (never commit):

```bash
QUARKGATE_URL=http://localhost:8080
QUARKGATE_KEY=qg_test_...
OPENROUTER_API_KEY=...
APIFY_TOKEN=...
LETTA_AGENT_ID=...
```

```bash
make e2e-live
```

Live tests use build tag `e2e` and are **not** run in default CI.
