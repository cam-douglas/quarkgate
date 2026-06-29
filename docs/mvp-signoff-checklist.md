# MVP sign-off checklist

- [x] Phase 3 gate: worker retry/DLQ, reconciliation fix, audit linkage, admin billing, metrics
- [x] `make verify-mvp` runs unit + driver contract tests
- [x] `scripts/e2e-smoke.sh` with `INTEGRATION=1` for metering loop
- [x] E2E orchestrator with status assertions
- [x] Edge protocols: soft-cap, terminal SSE error, Retry-After on 502/503
- [x] Bulkhead, graceful shutdown drain, DLQ replay (`admin replay-dlq`)
- [ ] Live provider E2E with sandbox credentials (manual)
- [ ] 1000-request reconciliation load test (manual / staging)
- [ ] Streaming p95 latency vs direct benchmark on staging hardware

**Operator decisions still open:** live E2E in CI (D4), Apify poll in proxy vs worker (D5).
