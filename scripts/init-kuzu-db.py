#!/usr/bin/env python3
"""Apply Kùzu schema and optional demo graph seed."""
from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

BRIDGE_ROOT = Path(__file__).resolve().parents[1] / "services" / "kuzu-bridge"
sys.path.insert(0, str(BRIDGE_ROOT))

from quark_kuzu_bridge.store import KuzuGraphStore  # noqa: E402


def main() -> None:
    parser = argparse.ArgumentParser(description="Initialize QuarkGate Kùzu database")
    parser.add_argument("--seed-demo", action="store_true", help="Insert demo episodes/skills")
    args = parser.parse_args()

    db_path = os.environ.get("KUZU_DB_PATH", "data/kuzu/memory_graph.kuzu")
    store = KuzuGraphStore(db_path)
    applied = store.init_schema()
    print(f"schema applied ({len(applied)} statements) -> {store.db_path}")

    if args.seed_demo:
        existing = store.query_whitelist("recent_episode_chain", {"limit": 1})
        if existing:
            print("demo seed skipped (episodes already present)")
        else:
            store.add_node("Episode", {"id": "demo-e1", "ts": "2026-06-30T12:00:00Z"})
            store.add_node("Episode", {"id": "demo-e2", "ts": "2026-06-29T12:00:00Z"})
            store.add_edge("demo-e1", "demo-e2", "FOLLOWS")
            store.add_node("Skill", {"id": "demo-s1", "name": "memory_search", "usage_count": 12})
            store.add_node("Entity", {"id": "demo-user", "name": "operator"})
            print("demo seed written")


if __name__ == "__main__":
    main()
