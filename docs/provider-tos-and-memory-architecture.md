# Provider TOS vs QuarkGate vs quarkOS memory architecture

**Status:** ACCEPTED credential policy → [ADR-036 BYOK](adr/036-byok-credential-model.md)  
**Last reviewed:** 2026-06-30 (public ToS only; not legal advice)

## Summary

| Question | Answer |
|----------|--------|
| **Interim product goal** | **BYOK** — each tenant uses **their own** provider keys until partnerships ([ADR-036](adr/036-byok-credential-model.md)). |
| Can you build **quarkOS memory architecture**? | **Yes** — direct integration; keys in **`quarkOS/.env`**. |
| Can you run **QuarkGate POC**? | **Yes** — sole-tenant BYOK, localhost, your sandbox keys. |
| Can you **commercialize master-key resale**? | **Not yet** — needs provider partnerships / Supabase for Platforms. |

---

## Two different models (do not conflate)

### A — quarkOS memory architecture (docs/ADR-032)

- **Your** platform embeds LLM, memory, and tools into **your product**.
- **Your** API keys (or **your users’ BYOK** where supported) call providers **directly** from quarkOS services.
- Metering via **cost-sentinel** and programme budgets — not reselling raw provider API access.
- Documented in `docs/04-data-and-memory-governance.md`, ADR-028/032/033/034, WP11 plan.

**TOS posture:** Normal SaaS / builder use — permitted by most providers when you comply with their acceptable-use and data rules.

### B — QuarkGate proxy + micro-billing (quarkgate/) — **BYOK interim (ADR-036)**

- End users hold **`qg_*` keys**; **their** downstream credentials live in vault (BYOK).
- **POC now:** operator is the **sole tenant** — your keys, localhost, no external signup.
- **Deferred:** operator master-key injection for paying third parties until **partnership** mode.
- Metering/holds still prove product viability without TOS-problematic resale.

**TOS posture:** BYOK + single-tenant POC = acceptable dev path; multi-tenant master-key resale = not until contracted.

---

## Provider-by-provider (public ToS)

### OpenRouter — **multi-tenant QuarkGate: NOT permitted**

OpenRouter Terms prohibit:

> access the Site or Service for purposes of **reselling API access to Models** or otherwise **developing a competing service**

Sources: [openrouter.ai/terms](https://openrouter.ai/terms) (§ prohibited uses).

**Allowed:** Build a SaaS or agent product **powered by** OpenRouter where users consume **your application’s capabilities**, not raw OpenRouter API access.

**Not allowed:** QuarkGate-style gateway where customers buy credits and receive OpenAI-compat proxy access to models.

**quarkOS memory path:** `OPENROUTER_API_KEY` in root `.env` for Archon / RSI / coding-agent **internal** calls — OK for operator’s product.

---

### Apify — **multi-tenant QuarkGate: NOT permitted without approval**

Apify Acceptable Use Policy prohibits:

> **resale of any Platform features without obtaining Apify’s prior written approval**

Also restricts proxy-server resale patterns in historical GTC.

Sources: [docs.apify.com/legal/acceptable-use-policy](https://docs.apify.com/legal/acceptable-use-policy)

**Allowed:** Your platform runs Apify actors **for your product** using **your** Apify account.

**Not allowed:** Billing third parties per compute second through an unsanctioned Apify proxy layer.

**Action if needed:** Contact Apify for written approval before QuarkGate Apify driver in production multi-tenant mode.

---

### Supabase — **multi-tenant QuarkGate: NOT on standard terms**

Supabase Terms prohibit:

> rent, lease, lend, sell, license, **sublicense**, assign, distribute, publish, transfer, or otherwise **make available the Services … to any third party**

Also prohibits competitive / commercial-disadvantage use of Supabase IP.

Sources: [supabase.com/terms](https://supabase.com/terms)

**Allowed for multi-tenant backends:** [Supabase for Platforms](https://supabase.com/docs/guides/integrations/supabase-for-platforms) (Management API, project claim, Platform Kit).

**quarkOS memory path:** Single **operator-owned** Supabase project for pgvector L3 — OK. QuarkGate `supabase` driver proxying **your** project to **external** `qg_*` customers — **not OK** on standard terms.

---

### Letta — **cloud proxy: unclear; self-host: preferred**

- **Letta Cloud:** Terms allow building agents **into your applications** ([letta.com/terms-of-service](https://www.letta.com/terms-of-service)). No explicit white-label API resale programme documented.
- **Self-hosted Letta (Docker):** Open source ([letta-ai/letta](https://github.com/letta-ai/letta)); no cloud API proxy needed for L0/L1.

**quarkOS memory path (ADR-032):** `LETTA_ENABLED=true` + `services/letta-bridge/` — direct bridge, not QuarkGate resale.

**QuarkGate `letta` driver:** Treat as **internal/dev only** unless Letta confirms multi-tenant proxy is acceptable.

---

### Zep Cloud (getzep.com) — **embed OK; resale gray**

Zep website terms restrict sell/lease/transfer of the service without permission ([getzep.com/legal/website-terms](https://www.getzep.com/legal/website-terms/)).

**quarkOS memory path (ADR-034):** `ZEP_API_KEY` + async ingest worker for **your** application’s episodic layer — standard embed pattern.

**Not documented:** QuarkGate-style rebilling of Zep API calls to third parties.

---

### OpenAI (embeddings) — **direct use only**

When `EMBED_PROVIDER=openai`, use `OPENAI_API_KEY` for **your** embed-worker — not via QuarkGate resale.

---

### Anytype — **local JSON-RPC only**

No cloud API key; local desktop + `ANYTYPE_RPC_URL`. Read-only agent bridge per runbook.

---

### Kùzu / LanceDB / CLIP — **local, no third-party TOS**

Embedded stores under `data/`; `CLIP_ENABLED=true` may use local model weights (owner hardware).

---

## Architecture alignment (what docs actually specify)

quarkOS **does not document** routing memory providers through QuarkGate (`docs/` has zero QuarkGate references). Integration mirror at `.cursor/quarkgate/integration-with-quarkos.md` marks cross-routing as **IMPLEMENTED_NOT_VERIFIED**.

| Memory layer | Technology | Keys / config | Via QuarkGate? |
|--------------|------------|---------------|----------------|
| L0/L1 | Letta + Hindsight | `LETTA_*`, self-host or cloud | **No** — letta-bridge |
| L2 episodic | Zep Cloud / Graphiti | `ZEP_API_KEY`, `ZEP_MODE` | **No** — zep-ingest-worker |
| L2 graph | Kùzu | local path | **No** |
| L3 semantic | pgvector + LanceDB | `SUPABASE_*`, `LANCE_ENABLED`, `EMBED_*` | **No** — memory-curator |
| L3 multimodal | CLIP | `CLIP_ENABLED` | **No** |
| L4 | Vault + agents.mmd | `VAULT_PATH`, `AGENTS_MMD_DATA` | **No** |
| HITL | Anytype | `ANYTYPE_*` | **No** |
| Cold path | Sleep worker | `SLEEP_CYCLE_ENABLED` | **No** |
| LLM (Archon, etc.) | OpenRouter / adapters | `OPENROUTER_API_KEY` | Optional future; **not** multi-tenant resale |

---

## Recommended env file split

| File | Purpose |
|------|---------|
| **`quarkOS/.env`** | Primary — full memory spectrum + programme services |
| **`quarkgate/.env`** | Optional — only if running QuarkGate for **internal** metering/dev; see TOS above |

---

## QuarkGate production options

1. **BYOK (now — ADR-036)** — per-tenant keys; POC = operator only.
2. **Partnership mode (later)** — written approval per provider; optional master keys.
3. **Direct integration** — quarkOS provider-adapters without QuarkGate (memory path).
4. ~~Master-key multi-tenant resale~~ — **withdrawn** until legal + provider sign-off.

---

## Before entering provider keys

1. Decide **single-tenant product** vs **multi-tenant API marketplace**.
2. If multi-tenant + QuarkGate credits → **stop** and get legal + provider approval first.
3. If single-tenant quarkOS memory spectrum → fill **`quarkOS/.env`** per `.env.example`; QuarkGate keys optional for local smoke only.

**UNVERIFIED:** Enterprise/reseller programmes may exist beyond public ToS — confirm with provider sales before relying on them.
