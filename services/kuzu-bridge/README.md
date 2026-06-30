# kuzu-bridge

Embedded **Kùzu** L2 graph service for QuarkGate (WP11 memory spectrum).

## Setup

From `quarkgate/`:

```bash
make setup-kuzu    # venv, PyPI kuzu 0.11.3, schema + demo seed
make kuzu-bridge   # HTTP on :8093 (set KUZU_ENABLED=true in .env)
make test-kuzu
```

Build Python bindings from your fork instead of PyPI:

```bash
KUZU_BUILD_FROM_SOURCE=1 make setup-kuzu
```

Fork lives at `tools/repos/kuzu` ([cam-douglas/kuzu](https://github.com/cam-douglas/kuzu)).

## HTTP API

| Route | Description |
|-------|-------------|
| `GET /health` | Service + DB path + whitelist |
| `POST /init` | Re-apply `schema.cypher` |
| `GET /graph/query?name=...` | Whitelisted Cypher queries |
| `POST /graph/nodes` | Insert node |
| `POST /graph/edges` | Insert edge |

Whitelisted queries: `recent_episode_chain`, `entity_degree`, `skill_usage_rank`.

## Driver

`drivers/kuzu/` exposes local bridge ops through the QuarkGate driver IPC (zero-cost, no upstream HTTP billing).

## Docker

```bash
docker compose up -d kuzu-bridge
```
