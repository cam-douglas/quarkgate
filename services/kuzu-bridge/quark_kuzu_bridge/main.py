from __future__ import annotations

import os
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI, HTTPException, Query

from quark_kuzu_bridge.store import KuzuGraphStore, WHITELIST_QUERIES

_store: KuzuGraphStore | None = None


def _enabled() -> bool:
    return os.environ.get("KUZU_ENABLED", "false").lower() in {"1", "true", "yes"}


def get_store() -> KuzuGraphStore:
    if _store is None:
        raise RuntimeError("Kùzu store not initialized")
    return _store


@asynccontextmanager
async def lifespan(_app: FastAPI):
    global _store
    if _enabled():
        _store = KuzuGraphStore()
        _store.init_schema()
    yield
    _store = None


app = FastAPI(title="kuzu-bridge", version="0.1.0", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, Any]:
    if not _enabled():
        return {"status": "disabled", "service": "kuzu-bridge", "kuzu_enabled": False}
    store = get_store()
    return {
        "status": "ok",
        "service": "kuzu-bridge",
        "kuzu_enabled": True,
        "db_path": str(store.db_path),
        "whitelist_queries": sorted(WHITELIST_QUERIES),
    }


@app.post("/init")
def init_schema() -> dict[str, Any]:
    if not _enabled():
        raise HTTPException(status_code=503, detail="KUZU_ENABLED is false")
    store = get_store()
    applied = store.init_schema()
    return {"initialized": True, "db_path": str(store.db_path), "statements": len(applied)}


@app.get("/graph/query")
def graph_query(name: str = Query(...), entity_id: str | None = None, limit: int = 5) -> dict[str, Any]:
    if not _enabled():
        raise HTTPException(status_code=503, detail="KUZU_ENABLED is false")
    if name not in WHITELIST_QUERIES:
        raise HTTPException(status_code=400, detail="Query not in whitelist")
    params: dict[str, Any] = {"limit": limit}
    if entity_id:
        params["entity_id"] = entity_id
    rows = get_store().query_whitelist(name, params)
    return {"query": name, "rows": rows}


@app.post("/graph/nodes")
def add_node(body: dict[str, Any]) -> dict[str, Any]:
    if not _enabled():
        raise HTTPException(status_code=503, detail="KUZU_ENABLED is false")
    label = str(body.get("label", "Episode"))
    props = dict(body.get("props", {}))
    node_id = get_store().add_node(label, props)
    return {"id": node_id, "label": label}


@app.post("/graph/edges")
def add_edge(body: dict[str, Any]) -> dict[str, str]:
    if not _enabled():
        raise HTTPException(status_code=503, detail="KUZU_ENABLED is false")
    get_store().add_edge(
        str(body["src"]),
        str(body["dst"]),
        str(body.get("rel", "FOLLOWS")),
        str(body.get("src_label", "Episode")),
        str(body.get("dst_label", "Episode")),
    )
    return {"status": "ok"}
