from __future__ import annotations

import os
import tempfile
from pathlib import Path

import pytest

from quark_kuzu_bridge.store import KuzuGraphStore


@pytest.fixture
def store() -> KuzuGraphStore:
    path = Path(tempfile.mkdtemp()) / "test.kuzu"
    os.environ["KUZU_DB_PATH"] = str(path)
    graph = KuzuGraphStore(str(path))
    graph.init_schema()
    return graph


def test_recent_episode_chain(store: KuzuGraphStore) -> None:
    store.add_node("Episode", {"id": "e1", "ts": "2026-06-30"})
    store.add_node("Episode", {"id": "e2", "ts": "2026-06-29"})
    rows = store.query_whitelist("recent_episode_chain", {"limit": 1})
    assert len(rows) == 1
    assert rows[0]["id"] == "e1"


def test_skill_usage_rank(store: KuzuGraphStore) -> None:
    store.add_node("Skill", {"id": "s1", "name": "a", "usage_count": 1})
    store.add_node("Skill", {"id": "s2", "name": "b", "usage_count": 5})
    rows = store.query_whitelist("skill_usage_rank", {"limit": 2})
    assert rows[0]["usage_count"] >= rows[1]["usage_count"]
