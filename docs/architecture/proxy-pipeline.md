# QuarkGate Proxy Pipeline

## Canonical middleware order (edge)

Outer → inner execution order:

1. **RequestID** — `X-Request-Id`, trace context
2. **Auth** — Bearer `qg_live_*` / `qg_test_*`; Redis `key:{hmac}` cache with PG fallback
3. **RateLimit** — Redis token bucket per user (`bucket:{user_id}:rpm`)
4. **Route + Transform** — resolve path/envelope; driver `prepareRequest`; vault credential fetch
5. **Idempotency** — `Idempotency-Key` replay for non-streaming; `409` for streaming
6. **Scope** — key scopes vs `provider` / `provider:operation`
7. **Solvency** — balance check; PG hold + Redis `hold:{request_id}` + `metering_sessions`
8. **Proxy** — downstream forward, SSE passthrough, metering event emit

Public routes **outside** this chain: `/healthz`, `/readyz`.

## Deviation from architecture plan diagram

The original plan diagram places **Solvency before Route**. Implementation keeps **Route before Solvency** because hold amount requires driver `estimate_micro` from resolved provider/operation.

## Streaming (SSE)

- `http.ResponseController` clears write deadline for streaming responses
- `meteringReader` flushes per chunk; `X-Accel-Buffering: no`
- Keepalive comments after `STREAM_IDLE_SEC` (default 30s)
- Max stream duration `STREAM_MAX_SEC` (default 1800s)
- Client disconnect cancels downstream via request context → `partial` metering
- Usage: provider `usage` in final SSE chunk, or fallback `chunk_estimate`

## Idempotency

- Header: `Idempotency-Key`
- Redis `idem:{user_id}:{key}:{path}` stores status + body for 24h
- Ledger capture/release uses client idempotency key when present

## Redis keys

| Key | Purpose |
|-----|---------|
| `key:{hmac}` | API key metadata cache |
| `keyid:{key_id}` | HMAC index for revoke invalidation |
| `balance:{user_id}` | Cached credit balance |
| `hold:{request_id}` | Active hold amount |
| `idem:{...}` | Idempotent response cache |
| `meter:stream` | Async metering events |

## Related code

- Gateway wiring: [`cmd/gateway/main.go`](../../cmd/gateway/main.go)
- Middleware: [`internal/gateway/middleware.go`](../../internal/gateway/middleware.go)
- Idempotency: [`internal/gateway/idempotency.go`](../../internal/gateway/idempotency.go)
- Proxy + stream meter: [`internal/proxy/handler.go`](../../internal/proxy/handler.go), [`internal/proxy/stream_meter.go`](../../internal/proxy/stream_meter.go)
