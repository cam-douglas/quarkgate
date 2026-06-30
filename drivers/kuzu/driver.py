"""QuarkGate driver — local kuzu-bridge HTTP proxy (no upstream billing)."""

from __future__ import annotations

import json
import os
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


def _base_url() -> str:
    return os.environ.get("KUZU_BRIDGE_URL", "http://127.0.0.1:8093").rstrip("/")


def _get(path: str, params: dict[str, Any] | None = None) -> tuple[int, str]:
    url = f"{_base_url()}{path}"
    if params:
        url = f"{url}?{urlencode(params)}"
    req = Request(url, method="GET")
    with urlopen(req, timeout=10) as resp:  # noqa: S310
        return resp.status, resp.read().decode()


def prepareRequest(ctx: dict[str, Any]) -> dict[str, Any]:
    envelope = ctx.get("envelope", {})
    payload = envelope.get("payload", {})
    op = envelope.get("operation", "graph.query")
    if op == "graph.query":
        params = {
            "name": payload.get("query_name", "recent_episode_chain"),
            "limit": payload.get("limit", 5),
        }
        if payload.get("entity_id"):
            params["entity_id"] = payload["entity_id"]
        status, body = _get("/graph/query", params)
        return {
            "method": "LOCAL",
            "url": f"{_base_url()}/graph/query",
            "headers": {},
            "body": None,
            "streaming": False,
            "provider": "kuzu",
            "operation": op,
            "estimated_cost_micro": estimateMaxCost(envelope),
            "local_status": status,
            "local_body": body,
        }
    if op == "graph.health":
        status, body = _get("/health")
        return {
            "method": "LOCAL",
            "url": f"{_base_url()}/health",
            "headers": {},
            "body": None,
            "streaming": False,
            "provider": "kuzu",
            "operation": op,
            "estimated_cost_micro": 0,
            "local_status": status,
            "local_body": body,
        }
    raise ValueError(f"unsupported operation: {op}")


def estimateMaxCost(envelope: dict[str, Any]) -> int:
    return 0


def normalizeUsage(raw: dict[str, Any], envelope: dict[str, Any] | None = None) -> dict[str, Any]:
    _ = envelope
    return {"api_calls": raw.get("api_calls", 1), "compute_seconds": 0}


def parseResponse(ctx: dict[str, Any]) -> dict[str, Any]:
    body = ctx.get("local_body") or ctx.get("body") or "{}"
    try:
        data = json.loads(body)
    except json.JSONDecodeError:
        data = {"raw": body}
    return {"api_calls": 1, "response": data}


def healthCheck(ctx: dict[str, Any]) -> dict[str, Any]:
    _ = ctx
    try:
        status, body = _get("/health")
        ok = status == 200 and "ok" in body
        return {"ok": ok, "latency_ms": 0, "message": body[:200]}
    except (HTTPError, URLError, TimeoutError) as exc:
        return {"ok": False, "latency_ms": 0, "message": str(exc)}
