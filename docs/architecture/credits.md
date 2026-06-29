# Credit unit

1 QuarkGate Credit = **1,000,000 micro_credits** (integer ledger).

Launch default: **1 Credit ≈ $0.01 USD**, configured via `CREDIT_USD_MICRO=10000` (10,000 USD micro-dollars per credit).

OpenRouter USD passthrough:

```
micro_credits = cost_usd * 1e6 * (1 + platform_margin) * CREDIT_USD_MICRO / 1e6
```

Provider rate tables live in `provider_configs.pricing_model` and `drivers/*/manifest.json`.
