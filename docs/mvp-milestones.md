# MVP Milestones M0–M9

| ID | Status | Verification |
|----|--------|--------------|
| M0 Foundation | Verified | `make test-infra`, docker-compose PG+Redis |
| M1 Identity | Verified | Auth middleware + admin CLI |
| M2 Solvency | Verified | 402 before downstream |
| M3 OpenRouter | Partial | Driver + proxy; live E2E manual (`make e2e-live`) |
| M4 Ledger | Verified | Worker capture/release + integration test (`INTEGRATION=1`) |
| M5 Apify | Partial | Poll driver; live manual |
| M6 Letta | Partial | Driver; live manual |
| M7 Supabase | Partial | Driver; live manual |
| M8 E2E orchestrator | Verified | `examples/swarm-minimal/orchestrator.js` with assertions |
| M9 Hardening | Partial | Circuit breaker, bulkhead, DLQ replay, chaos tests |

Run `make verify-mvp` for automated checks.
