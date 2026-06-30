# ADR-001: Go edge + TypeScript driver host

**Status:** Accepted

Go handles HTTP proxy, auth, solvency, and metering emit. Provider transforms run in Node/Python subprocess IPC (`drivers/sdk/host.js`).

# ADR-002: Micro-credits integer ledger

**Status:** Accepted

All monetary values are `BIGINT micro_credits` (1 credit = 1,000,000 micro). No floats in ledger.

# ADR-003: Hold / capture / release

**Status:** Accepted

Pessimistic hold at request start; async worker captures actual usage and releases remainder.

# ADR-004: Redis Streams before Kafka

**Status:** Accepted

`meter:stream` + consumer group for MVP metering queue.

# ADR-005: Subprocess driver IPC before WASM

**Status:** Accepted

One-shot `node host.js` per IPC call; process pool deferred.

# ADR-036: BYOK credential model (interim)

**Status:** Accepted

Bring-your-own-key until provider partnerships. See [036-byok-credential-model.md](036-byok-credential-model.md).
