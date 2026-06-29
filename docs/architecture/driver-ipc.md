# Driver IPC Protocol

QuarkGate invokes provider drivers via **JSON stdin / stdout** through `drivers/sdk/host.js` (or `host.py`).

**Version:** `ipc_version: "1"`

## Process pool

Driver process pooling is **deferred**. Each IPC call spawns `node drivers/sdk/host.js` (one-shot). Expected overhead: ~20–50ms per call on warm systems. Set `DRIVER_POOL_SIZE` support planned for post-Phase 4.

## Actions

### `prepare`

**Input:**
```json
{
  "ipc_version": "1",
  "action": "prepare",
  "provider": "openrouter",
  "envelope": { ... },
  "credential": "sk-...",
  "baseURL": "https://..."
}
```

**Output:** `DownstreamRequest` JSON (`url`, `method`, `headers`, `body`, `streaming`, `estimate_micro`, ...)

### `estimate`

**Input:** `action`, `provider`, `envelope`

**Output:** `{ "estimate_micro": 1000000 }`

### `normalize`

**Input:** `action`, `provider`, `raw_usage`, `envelope` (optional)

**Output:** `{ "raw_usage": { ... } }` — patches merged by Go worker before rate-table normalize

### `parseResponse`

**Input:** `action`, `provider`, `headers`, `body` (string), `streaming` (bool), `envelope` (optional)

**Output:** `{ "raw_usage": { ... } }`

### `healthCheck`

**Input:** `action`, `provider`, `baseURL`, `credential`

**Output:** `{ "ok": true, "latency_ms": 42, "message": "" }`

### `poll` (Apify async)

**Input:** `action`, `provider`, `baseURL`, `credential`, `poll_context` (e.g. `run_id`)

**Output:** `{ "raw_usage": { "compute_seconds": 45 }, "done": true }`

## Errors

stderr: `{ "error": "message", "code": "DRIVER_ERROR" }`  
exit code: 1
