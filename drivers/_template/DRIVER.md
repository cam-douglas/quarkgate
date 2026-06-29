# QuarkGate Driver Template

Copy this folder to `drivers/<your-provider>/` and implement `driver.js`.

See [docs/architecture/drivers.md](../../docs/architecture/drivers.md) for the full contributor guide.

## Files

- `manifest.json` — operations, compat paths, pricing hints, capabilities
- `driver.js` — full driver contract exports
- `DRIVER.md` — optional human notes

## driver.js exports

```javascript
function prepareRequest({ envelope, credential, baseURL }) { ... }
function estimateMaxCost(envelope) { ... }
function normalizeUsage(raw, envelope) { ... }
function parseResponse({ headers, body, streaming, envelope }) { ... }
async function healthCheck({ baseURL, credential }) { ... }
module.exports = { prepareRequest, estimateMaxCost, normalizeUsage, parseResponse, healthCheck };
```

## Testing

```bash
npm run validate:manifests
npm run test:drivers
```

Add golden fixtures under `drivers/fixtures/<provider-id>/` for each action you implement.
