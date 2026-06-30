# QuarkGate owner decisions (suggested defaults)

Record authoritative choices here. Agents should treat filled rows as final.

| ID | Decision | Suggested choice | Rationale |
|----|----------|------------------|-----------|
| D4 | Live E2E in CI | **A+C** | Mock-only CI; manual strict orchestrator for live spend |
| D5 | Apify poll location | **A** | Proxy blocks until complete — matches current driver + orchestrator |
| D2 | Solvency hot-path | **A** | Sync PG hold (MVP default) |
| D3 | Reconciliation auto-fix | **Dev/staging on, prod manual** | `RECONCILE_AUTO_FIX=true` locally; manual `reconcile-user` in prod |
| D7 | 402 top_up_url | **Omit until dashboard** | No credit top-up product in BYOK POC |
| D8 | Observability | **Prometheus only (MVP)** | Defer OTel until staging |
| D9 | Letta MVP surface | **agents.messages.create only** | Matches driver scope |
| D10 | Driver IPC pool | **Defer** | Benchmark after load test |

## Your sign-off (fill when complete)

| ID | Your choice | Date | Notes |
|----|-------------|------|-------|
| D4 | A+C | | |
| D5 | A | | |
| D2 | A | | |
| D3 | staging auto / prod manual | | |

## MVP sign-off remaining (owner actions)

1. Run vault bootstrap: `bash quarkgate/scripts/bootstrap-vault-from-env.sh`
2. Ensure ledger worker running (supervisor or `make worker`)
3. Run strict E2E: `E2E_STRICT=1 node quarkgate/examples/swarm-minimal/orchestrator.js`
4. Record evidence in [owner-operator-checklist.md](owner-operator-checklist.md#evidence-log)
5. Optional: 1000-request reconciliation + p95 latency benchmark (staging)
