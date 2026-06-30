# QuarkGate — owner / operator checklist

**Audience:** Human owner, operator, or integrator (not implementation agents).  
**Purpose:** Close MVP sign-off gaps, record product decisions, and wire QuarkGate into quarkOS where needed.

**Related:**

- Engineering sign-off (automated): [mvp-signoff-checklist.md](mvp-signoff-checklist.md)
- Phase 5 decisions (detail): [decision-log-suggested.md](decision-log-suggested.md)
- quarkOS integration: [../../docs/integrations/quarkgate-quarkos.md](../../docs/integrations/quarkgate-quarkos.md)

---

## How to use this document

1. Work **top to bottom**. Later steps depend on earlier ones.
2. Check a box only when **you** have completed the step (not when code exists).
3. Record decisions in the **Decision log** section so agents and future you do not re-litigate.

**Credentials template:** [`../.env.example`](../.env.example) (QuarkGate only) and **[`../../.env.example`](../../.env.example)** (quarkOS memory spectrum — primary).

**TOS / architecture (read before keys):** [provider-tos-and-memory-architecture.md](provider-tos-and-memory-architecture.md)

---

## Step 0 — Provide credentials (owner → agent handoff)

**Goal:** Fill env files so the agent can bootstrap services and run verification.

**Architecture decision (required first):**

- [x] Read [ADR-036 BYOK](adr/036-byok-credential-model.md) and [provider-tos-and-memory-architecture.md](provider-tos-and-memory-architecture.md)
- [x] Confirm **BYOK + sole-tenant POC** (you = only `qg_*` user) until partnerships
- [x] **Do not** onboard external users or sell credits until `QUARKGATE_CREDENTIAL_MODE=partnership` per provider

**Primary — quarkOS memory spectrum (`quarkOS/.env`):**

- [x] Fill **`quarkOS/.env`** per WP11 plan ([phase 11](../../docs/plans/project_quark_phase_11_5e1f43aa.plan.md)):

  **Keys required [MEMORY POC]:**
  - [ ] `OPENAI_API_KEY` (L3 embed; set `EMBED_PROVIDER=openai`) — **N/A:** using `EMBED_PROVIDER=openrouter`
  - [x] `OPENROUTER_API_KEY` (Archon / Letta LLM hot path)
  - [x] `LETTA_API_KEY` + `LETTA_AGENT_ID` (or self-host `:8283`)
  - [x] `ZEP_API_KEY` (set `ZEP_MODE=cloud`)

  **Flags / paths [OPTIONAL — no cloud key]:**
  - [x] `LETTA_ENABLED`, `HINDSIGHT_*`, `AGENTS_MMD_DATA`
  - [ ] `GRAPHITI_BASE_URL` + `ZEP_MODE=local` for fallback — **deferred** (Zep Cloud active)
  - [x] `KUZU_*`, `LANCE_*`, `LANCEDB_URI`
  - [x] `CLIP_*`, `BROWSER_ACTORS_ENABLED` — enabled in dev (Playwright template pending)
  - [x] `ANYTYPE_*`, `LANGGRAPH_SKILLS_ENABLED`, `SLEEP_CYCLE_*`, `MEMORY_SLEEP_WORKER_URL`
  - [x] Service URLs: `EMBED_WORKER_URL`, `ZEP_INGEST_WORKER_URL`, `LETTA_BRIDGE_URL`

  **Already set if Supabase linked:** `SUPABASE_*`, `NEXT_PUBLIC_SUPABASE_*`

**QuarkGate BYOK smoke (`quarkgate/.env`):**

- [x] Set `QUARKGATE_CREDENTIAL_MODE=byok`
- [x] Fill **same provider keys** you own (OpenRouter, Apify, Letta, Supabase) for driver POC
- [x] Tell the agent: *"BYOK credentials in `.env` files — proceed with memory spectrum + QuarkGate smoke."*

**Agent will (after you provide `.env`):**

1. [x] `resolve-dev-infra.py && migrate` (no Docker; Supabase PG + host Redis)
2. [x] `store-credential` for each provider from env (`bootstrap-vault-from-env.sh`)
3. [x] Patch `provider_configs.base_url` from env
4. [x] `create-user`, `deposit-credits`, `create-key` → `QUARKGATE_KEY` in `.env`
5. [x] Run gateway + worker + strict orchestrator
6. [x] Record results in [Evidence log](#evidence-log)

---

## Step 1 — Product decisions (blocking)

- [x] **D4** — Live E2E in CI: **A+C** (mock CI + manual live)
- [x] **D5** — Apify poll: **A** (proxy blocks until complete)
- [x] **D2** — Solvency hot-path: **A** (sync PG hold)
- [x] **D3** — Reconciliation: auto-fix dev/staging; manual prod
- [x] Record choices in [Decision log](#decision-log) below

---

## Step 2 — Sandbox credentials & vault setup

- [x] Create or designate **sandbox accounts** for: OpenRouter, Apify, Letta, Supabase
- [ ] Set **spend caps / alerts** on each sandbox account (per D4 policy) — **owner: set in provider dashboards**
- [x] `cd quarkgate && migrate` (via `resolve-dev-infra.py`; no Docker)
- [x] Create dev user, deposit credits, create API key — **note:** `qg_live_*` sole-tenant POC (not `qg_test_*`)
- [x] Store downstream credentials via admin CLI (`bash scripts/bootstrap-vault-from-env.sh`)
- [x] `.env` aligned with [`.env.example`](../.env.example)

---

## Step 3 — Live provider E2E (manual)

- [x] Start gateway and worker (unified dev gateway + `ledger-worker`)
- [x] Resolve port conflict (task-service `:8081`, gateway `:8080`, unified `:8090`)
- [x] Run orchestrator in **strict** mode (`E2E_STRICT=1`)
- [ ] All four providers return **200** — **partial:** OpenRouter 200, Apify 201, Supabase 200 ✓, Letta **402** (Letta Cloud credits)
- [x] Confirm `usage_logs` captured (not pending) after ledger worker processes stream — verified 2026-06-30
- [ ] Update [mvp-signoff-checklist.md](mvp-signoff-checklist.md): Live provider E2E ✓ (blocked on Letta credits)
- [ ] Update milestone tracker when all four return 200

---

## Step 4 — Staging load & latency (manual)

- [ ] **1000-request reconciliation:** drift **< 0.01%**
- [ ] Document command/script and result
- [ ] **Streaming p95 latency:** p95 delta **≤ 50ms**
- [ ] Document p50/p95 numbers and hardware profile
- [ ] Update [mvp-signoff-checklist.md](mvp-signoff-checklist.md): load test ✓, latency benchmark ✓

---

## Step 5 — Production readiness (before any prod traffic)

- [ ] Replace dev-only **`VAULT_KEK`** with production key material
- [ ] Set production **`DATABASE_URL`**, **`REDIS_URL`**, **`LISTEN_ADDR`**
- [ ] Confirm **D3** prod policy documented in runbook
- [ ] Confirm **D7**: `402 top_up_url` omit until dashboard
- [ ] Operator runbook acknowledged (DLQ replay, reconcile-user)

---

## Step 6 — quarkOS integration (optional, post-MVP sign-off)

- [x] Decide: LLM/Apify via QuarkGate when `QUARKGATE_ENABLED=true`; memory stays direct
- [x] `QUARKGATE_URL` / `QUARKGATE_KEY` / `QUARKGATE_ENABLED` in quarkOS `.env` + `.env.example`
- [x] `packages/provider-adapters` QuarkGate client + `get_llm_adapter()`
- [ ] Align WP11 Letta/Zep cost reservation with QuarkGate holds
- [ ] Write quarkOS ADR: production topology (DO + QuarkGate edge)
- [x] Bump `quarkgate/` submodule pin if using submodule workflow — pending parent repo commit

---

## Step 7 — quarkOS Supabase (programme, not QuarkGate ledger)

Ledger uses same Supabase Postgres project in dev (no separate `:5433` Docker).

- [ ] `supabase login` — **CLI 403** (account lacks CLI privileges); DB access via `DATABASE_URL` works
- [x] Programme migrations applied (verified: `tasks`, `skill_schedules`, `hermes_*`, `memory_*`, ledger tables)
- [x] Schema current via `psql "$DATABASE_URL"` / `schema_migrations`

---

## Secondary decisions (non-blocking for MVP sign-off)

- [x] **D6** Stream soft-cap — no change needed
- [x] **D7** `402 top_up_url` — omit until dashboard
- [x] **D8** Observability — Prometheus only (MVP)
- [x] **D9** Letta — `agents.messages.create` only
- [x] **D10** Driver IPC pool — defer

---

## Decision log

| ID | Decision | Your choice | Date | Notes |
|----|----------|-------------|------|-------|
| D4 | Live E2E in CI | A+C | 2026-06-30 | Mock CI; manual strict orchestrator |
| D5 | Apify poll location | A | 2026-06-30 | Proxy blocks until complete |
| D2 | Solvency hot-path | A | 2026-06-30 | Sync PG hold |
| D3 | Reconciliation auto-fix (prod) | staging auto / prod manual | 2026-06-30 | `RECONCILE_AUTO_FIX=true` locally |
| D7 | 402 top_up_url | omit | 2026-06-30 | No top-up product in BYOK POC |
| D8 | Observability stack | Prometheus MVP | 2026-06-30 | Defer OTel |
| D9 | Letta MVP surface | messages.create | 2026-06-30 | |
| D10 | Driver IPC pool | defer | 2026-06-30 | |

---

## Evidence log (live / load runs)

| Run | Date | Environment | Result | Notes |
|-----|------|-------------|--------|-------|
| Live E2E orchestrator (strict) | 2026-06-30 | local | partial | OR 200, Apify 201, SB 200, Letta 402 |
| Supabase match_documents RPC | 2026-06-30 | Supabase | pass | migration `20250630143000_quarkgate_match_documents.sql` |
| usage_logs metering | 2026-06-30 | local | pass | completed rows for OR/Apify/SB; Letta failed+ captured |
| Vault bootstrap + driver health | 2026-06-30 | local | pass | 4/4 driver-health ok |
| Dev gateway API test | 2026-06-30 | local | pass | 27/27 probes up |
| 1000-request reconciliation | | staging | | |
| Streaming p95 latency | | staging | | |

---

## Step 1 — Product decisions (blocking)

See [decision-log-suggested.md](decision-log-suggested.md) for rationale. Choices recorded above.

---

## After Step 1

Proceed to close Step 3 partial E2E (**Letta 402 only** — add Letta Cloud credits) and Step 4 load/latency when ready for staging sign-off.
