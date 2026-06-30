# QuarkGate implementation plans

Phased engineering plans for building QuarkGate. Each phase has a dedicated plan file; the master reference for all phases is **Phase 1**.

| Phase | File | Scope |
|-------|------|--------|
| 1 | [phase_1_foundation.plan.md](phase_1_foundation.plan.md) | Stack, schema, vault, admin CLI, full architecture (Phases 4–5) |
| 2 | [phase_2_proxy_auth.plan.md](phase_2_proxy_auth.plan.md) | Reverse proxy, middleware, SSE streaming, auth |
| 3 | [phase_3_metering.plan.md](phase_3_metering.plan.md) | Async metering, ledger worker, normalization, reconciliation |
| 4 | [phase_4_drivers.plan.md](phase_4_drivers.plan.md) | Pluggable driver architecture, IPC contract, golden tests |
| 5 | [phase_5_mvp_hardening.plan.md](phase_5_mvp_hardening.plan.md) | MVP milestones M0–M9 verification, E2E, chaos, runbooks |

**Convention:** `phase_<N>_<topic>.plan.md` — no hash suffixes.

**Credential policy:** [ADR-036 BYOK](adr/036-byok-credential-model.md) — interim product goal until provider partnerships.

**Status:** Phase 1–2 and Phase 4 implemented; Phase 3 and Phase 5 plans ready — **complete Phase 3 before Phase 5 sign-off**.
