# Metering pipeline

## Flow

1. **Edge** — After downstream response, proxy emits `MeteringEvent` to Redis Stream `meter:stream` via `gateway.EmitMeterEvent`.
2. **Hold** — Solvency middleware places PG `hold` ledger row and Redis `hold:{request_id}` before downstream.
3. **Worker** — `ledger-worker` consumes consumer group `ledger-workers`, calls `metering.Worker.ProcessEvent`.
4. **Normalize** — Driver `normalizeUsage` (optional) then Go `metering.Normalize` / `NormalizeWithOptions`.
5. **Capture/release** — `store.CaptureAndRelease` writes ledger `capture` + `release` rows and updates `usage_logs`.
6. **Cache** — Redis `balance:{user_id}` synced from PG after capture.

## Reliability

- Failed processing: message **not ACKed** until `WORKER_MAX_RETRIES` deliveries, then `meter:dlq`.
- Replay: `go run ./cmd/admin replay-dlq` or `go run ./cmd/ledger-worker --replay-dlq`.
- Reconciliation: hourly job compares ledger sum vs `users.credit_balance_micro`; `RECONCILE_AUTO_FIX=true` repairs from ledger.

## Metrics

- Gateway: `/metrics`
- Worker: `:9091/metrics` (default `WORKER_METRICS_ADDR`)
