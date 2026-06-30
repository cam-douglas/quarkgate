# ADR-036: BYOK credential model (interim product goal)

**Status:** ACCEPTED  
**Date:** 2026-06-30  
**Review date:** When first provider partnership signed

## Context

QuarkGate was designed as a unified proxy with **operator master credentials** injected for all `qg_*` customers and **credit-based billing**. Public provider ToS (OpenRouter, Apify, standard Supabase) restrict **API resale** and **competing aggregation** without written approval.

quarkOS memory architecture (ADR-032) integrates providers **directly** via operator or user keys — the compliant baseline.

## Decision

Until official **provider partnership / reseller / platform** agreements exist:

1. **Interim product goal:** QuarkGate operates in **BYOK (Bring Your Own Key)** mode.
2. Each tenant (for POC: **the operator only**) stores **their own** downstream credentials in QuarkGate vault — linked to their `qg_*` key — not shared operator master keys for third parties.
3. **quarkOS memory providers** (Zep, embed-worker, letta-bridge, etc.) remain in **`quarkOS/.env`** with **direct** integration — not routed through QuarkGate for MVP/POC.
4. **QuarkGate drivers** (OpenRouter, Apify, Letta, Supabase) are for **POC metering/routing smoke** using **sole-tenant BYOK** (operator = only user).
5. **Deferred:** Multi-tenant master-key resale, external credit top-up product, and public `qg_live_*` marketplace.

## Credential modes (roadmap)

| Mode | When | Credentials |
|------|------|-------------|
| **`byok`** | **Now (POC / pre-partnership)** | Per-tenant keys in vault; operator is first tenant |
| **`partnership`** | After written provider approval | Contracted master keys + permitted billing |
| ~~`master`~~ | **Withdrawn for production** | Operator master key for all users — TOS conflict |

Env flag (documentation / future code): `QUARKGATE_CREDENTIAL_MODE=byok`

## POC scope (permitted dev use)

- Localhost / private staging
- Single operator as sole `qg_test_*` tenant
- Sandbox provider keys with spend caps
- Prove auth, holds, metering, drivers, E2E — **not** external signup or credit sales

## Commercial path (later)

1. Execute provider programmes (OpenRouter, Apify, Supabase for Platforms, etc.)
2. Legal review of billing model
3. Introduce `partnership` mode per provider driver
4. Optional: platform fee on BYOK routing even before master-key resale

## Consequences

- MVP sign-off and live E2E use **operator BYOK** credentials only
- Documentation and `.env.example` files reflect BYOK as default
- Implementation backlog: per-user credential binding in vault (today: admin `store-credential` simulates single tenant)

## References

- [provider-tos-and-memory-architecture.md](../provider-tos-and-memory-architecture.md)
- quarkOS [ADR-032](../../docs/adr/032-unified-memory-spectrum.md)
- [owner-operator-checklist.md](../owner-operator-checklist.md)
