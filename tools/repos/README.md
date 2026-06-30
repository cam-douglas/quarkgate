# Git repositories

Clone third-party or forked repos here. Each checkout lives in its own subdirectory.

## Checkouts

| Directory | Remote | Notes |
|-----------|--------|-------|
| `kuzu/` | https://github.com/cam-douglas/kuzu.git | Embedded graph DB (fork of [kuzudb/kuzu](https://github.com/kuzudb/kuzu)); WP11 L2 graph store |

Setup and bridge service: [`../scripts/setup-kuzu.sh`](../scripts/setup-kuzu.sh), [`../services/kuzu-bridge/`](../services/kuzu-bridge/).

## Clone / update

```bash
# From quarkgate/
git clone https://github.com/cam-douglas/kuzu.git tools/repos/kuzu

# Refresh an existing checkout
git -C tools/repos/kuzu pull
```
