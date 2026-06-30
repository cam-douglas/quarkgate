# QuarkGate tools

Local workspace for vendored git repositories, binaries, and helper tooling used by QuarkGate development and integration work.

## Layout

| Path | Purpose |
|------|---------|
| `repos/` | Git checkouts (forks, upstream mirrors, SDKs) |
| `bin/` | Optional local binaries or wrappers (not committed) |

Cloned repositories under `repos/` are **gitignored** — clone locally per `repos/README.md`. Do not commit full upstream trees into QuarkGate.
