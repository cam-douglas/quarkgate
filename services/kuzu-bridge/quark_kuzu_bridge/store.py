from __future__ import annotations

import os
from pathlib import Path
from typing import Any

import kuzu

WHITELIST_QUERIES: dict[str, str] = {
    "recent_episode_chain": (
        "MATCH (e:Episode) RETURN e.id AS id, e.ts AS ts ORDER BY e.ts DESC LIMIT $limit"
    ),
    "entity_degree": (
        "MATCH (n:Entity {id: $entity_id})-[r]-() RETURN n.id AS entity_id, count(r) AS degree"
    ),
    "skill_usage_rank": (
        "MATCH (s:Skill) RETURN s.id AS id, s.name AS name, s.usage_count AS usage_count "
        "ORDER BY s.usage_count DESC LIMIT $limit"
    ),
}


class KuzuGraphStore:
    """Embedded Kùzu graph store for WP11 L2 derived relations."""

    def __init__(self, db_path: str | None = None) -> None:
        path = db_path or os.environ.get("KUZU_DB_PATH", "data/kuzu/memory_graph.kuzu")
        self._path = Path(path)
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._db = kuzu.Database(str(self._path))
        self._conn = kuzu.Connection(self._db)

    @property
    def db_path(self) -> Path:
        return self._path

    def init_schema(self) -> list[str]:
        schema_file = Path(__file__).with_name("schema.cypher")
        applied: list[str] = []
        for line in schema_file.read_text(encoding="utf-8").splitlines():
            stmt = line.strip()
            if not stmt or stmt.startswith("//"):
                continue
            self._conn.execute(stmt)
            applied.append(stmt)
        return applied

    def add_node(self, label: str, props: dict[str, Any]) -> str:
        node_id = str(props.get("id", f"{label}:{self._count_nodes(label)}"))
        props = {**props, "id": node_id}
        cols = ", ".join(f"{k}: ${k}" for k in props)
        self._conn.execute(f"CREATE (n:{label} {{{cols}}})", props)
        return node_id

    def add_edge(self, src: str, dst: str, rel: str, src_label: str = "Episode", dst_label: str = "Episode") -> None:
        self._conn.execute(
            f"MATCH (a:{src_label} {{id: $src}}), (b:{dst_label} {{id: $dst}}) "
            f"MERGE (a)-[:{rel}]->(b)",
            {"src": src, "dst": dst},
        )

    def query_whitelist(self, name: str, params: dict[str, Any] | None = None) -> list[dict[str, Any]]:
        if name not in WHITELIST_QUERIES:
            raise ValueError(f"Query not in whitelist: {name}")
        bound = dict(params or {})
        if "limit" in bound:
            bound["limit"] = int(bound["limit"])
        query = WHITELIST_QUERIES[name]
        used = {k: bound[k] for k in bound if f"${k}" in query}
        result = self._conn.execute(query, used)
        rows: list[dict[str, Any]] = []
        columns = result.get_column_names()
        while result.has_next():
            values = result.get_next()
            rows.append(dict(zip(columns, values, strict=True)))
        return rows

    def _count_nodes(self, label: str) -> int:
        result = self._conn.execute(f"MATCH (n:{label}) RETURN count(n) AS c")
        if result.has_next():
            return int(result.get_next()[0])
        return 0
