# MVP sign-off checklist

- [x] Phase 3 gate: worker retry/DLQ, reconciliation fix, audit linkage, admin billing, metrics
- [x] `make verify-mvp` runs unit + driver contract tests
- [x] `scripts/e2e-smoke.sh` with `INTEGRATION=1` for metering loop
- [x] E2E orchestrator with status assertions
- [x] Edge protocols: soft-cap, terminal SSE error, Retry-After on 502/503
- [x] Bulkhead, graceful shutdown drain, DLQ replay (`admin replay-dlq`)
- [x] Vault bootstrap + driver health (4/4 providers) — `bootstrap-vault-from-env.sh`
- [x] Ledger worker running in dev stack
- [ ] Live provider E2E with sandbox credentials — **partial** (OR/Apify/SB pass; Letta 402 — owner credits later)
- [x] 1000-request reconciliation load test — `make load-reconciliation` PASS (local 2026-06-30)
- [x] Streaming p95 latency vs direct benchmark — `make streaming-p95` PASS (gateway :8080)

**Operator decisions:** D4 **A+C**, D5 **A** — recorded in [owner-operator-checklist.md](owner-operator-checklist.md#decision-log).

**Owner workflow:** [owner-operator-checklist.md](owner-operator-checklist.md)  
**TOS / architecture:** [provider-tos-and-memory-architecture.md](provider-tos-and-memory-architecture.md)  
**Credential policy:** [ADR-036 BYOK](adr/036-byok-credential-model.md)
