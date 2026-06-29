# QuarkGate driver contributor guide

QuarkGate routes API traffic through **provider drivers** — small modules that translate QuarkGate envelopes into downstream HTTP calls and extract usage for metering.

## Layout

Each driver lives in `drivers/<provider-id>/`:

| File | Purpose |
|------|---------|
| `manifest.json` | Registry metadata: operations, compat paths, pricing hints, capabilities |
| `driver.js` or `driver.py` | Driver implementation |
| `DRIVER.md` | Human documentation (optional) |

Golden contract tests use fixtures in `drivers/fixtures/<provider-id>/`.

## Driver contract

Export these functions from your driver (JavaScript or Python):

| Function | IPC action | Required |
|----------|------------|----------|
| `prepareRequest(ctx)` | `prepare` | Yes |
| `estimateMaxCost(envelope)` | `estimate` | Yes |
| `normalizeUsage(raw, envelope)` | `normalize` | Recommended |
| `parseResponse(ctx)` | `parseResponse` | Recommended |
| `healthCheck(ctx)` | `healthCheck` | Recommended |
| `pollRun(ctx)` | `poll` | Only for async providers (e.g. Apify) |

Set matching flags in `manifest.json`:

```json
"capabilities": {
  "parse_response": true,
  "normalize_usage": true,
  "async_poll": false
}
```

## IPC protocol

See [driver-ipc.md](./driver-ipc.md) for stdin/stdout JSON shapes per action.

Go invokes `node drivers/sdk/host.js` (or `python3 drivers/sdk/host.py` for Python-only drivers).

## Adding a driver

1. Copy `drivers/_template/` to `drivers/<your-id>/`.
2. Fill `manifest.json` — `id` must match the folder name.
3. Implement `driver.js` (or `driver.py`).
4. Add golden fixtures under `drivers/fixtures/<your-id>/`.
5. Run locally:

```bash
npm run validate:manifests
npm run test:drivers
go test ./internal/registry/...
```

## Async poll pattern (Apify)

For long-running jobs:

1. `prepareRequest` starts the run.
2. `parseResponse` returns `needs_poll: true` and `run_id`.
3. Edge calls `poll` until `done: true`; driver returns final `compute_seconds`.
4. Ledger worker applies `normalizeUsage` then Go `metering.Normalize`.

## Performance note

Each IPC call spawns a short-lived Node/Python process. A multiplexed driver process pool is deferred; see driver-ipc.md.
