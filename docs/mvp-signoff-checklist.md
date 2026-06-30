# MVP sign-off checklist

- [x] Phase 3 gate: worker retry/DLQ, reconciliation fix, audit linkage, admin billing, metrics
- [x] `make verify-mvp` runs unit + driver contract tests
- [x] `scripts/e2e-smoke.sh` with `INTEGRATION=1` for metering loop
- [x] E2E orchestrator with status assertions
- [x] Edge protocols: soft-cap, terminal SSE error, Retry-After on 502/503
- [x] Bulkhead, graceful shutdown drain, DLQ replay (`admin replay-dlq`)
- [x] Vault bootstrap + driver health (4/4 providers) — `bootstrap-vault-from-env.sh`
- [x] Ledger worker running in dev stack
- [ ] Live provider E2E with sandbox credentials — **partial** (OR 200, Apify 201, SB 200; Letta 402 credits)
- [ ] 1000-request reconciliation load test (manual / staging)
- [ ] Streaming p95 latency vs direct benchmark on staging hardware

**Operator decisions:** D4 **A+C**, D5 **A** — recorded in [owner-operator-checklist.md](owner-operator-checklist.md#decision-log).

**Owner workflow:** [owner-operator-checklist.md](owner-operator-checklist.md)  
**TOS / architecture:** [provider-tos-and-memory-architecture.md](provider-tos-and-memory-architecture.md)  
**Credential policy:** [ADR-036 BYOK](adr/036-byok-credential-model.md)
