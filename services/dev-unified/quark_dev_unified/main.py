from __future__ import annotations

import asyncio
import os
import sys
from contextlib import asynccontextmanager
from pathlib import Path
from typing import AsyncIterator

import httpx
from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse

# Supervisor lives in quarkgate/scripts/
_GATE = Path(__file__).resolve().parents[3]
if str(_GATE / "scripts") not in sys.path:
    sys.path.insert(0, str(_GATE / "scripts"))

import dev_supervisor as supervisor  # noqa: E402
from quark_dev_dashboard.probes import normalize_health, run_all_probes, run_api_test  # noqa: E402
from quark_dev_dashboard.main import _env_summary, templates  # noqa: E402
from fastapi import Request  # noqa: E402
from fastapi.responses import HTMLResponse  # noqa: E402

PROXY_PREFIXES: tuple[tuple[str, str], ...] = (
    ("/qg", os.environ.get("QUARKGATE_INTERNAL_URL", "http://127.0.0.1:8080")),
    ("/tasks", os.environ.get("TASK_API_URL", "http://127.0.0.1:8081")),
    ("/memory", os.environ.get("MEMORY_CURATOR_URL", "http://127.0.0.1:8082")),
    ("/exec", os.environ.get("EXECUTION_CONTROLLER_URL", "http://127.0.0.1:8083")),
    ("/hermes", os.environ.get("HERMES_URL", "http://127.0.0.1:8084")),
    ("/embed", os.environ.get("EMBED_WORKER_URL", "http://127.0.0.1:8087")),
    ("/letta", os.environ.get("LETTA_BRIDGE_URL", "http://127.0.0.1:8089")),
    ("/zep", os.environ.get("ZEP_INGEST_WORKER_URL", "http://127.0.0.1:8091")),
    ("/kuzu", os.environ.get("KUZU_BRIDGE_URL", "http://127.0.0.1:8093")),
)

_lock_handle: object | None = None


async def _reverse_proxy(request: Request, upstream_base: str, subpath: str) -> Response:
    base = upstream_base.rstrip("/")
    path = subpath.lstrip("/")
    url = f"{base}/{path}" if path else base
    if request.url.query:
        url = f"{url}?{request.url.query}"
    headers = {
        k: v
        for k, v in request.headers.items()
        if k.lower() not in ("host", "content-length", "transfer-encoding")
    }
    body = await request.body()
    try:
        async with httpx.AsyncClient(follow_redirects=True, timeout=120.0) as client:
            upstream = await client.request(request.method, url, headers=headers, content=body)
    except httpx.RequestError as exc:
        return Response(
            content=f'{{"error":"upstream unavailable","detail":"{exc}"}}',
            status_code=502,
            media_type="application/json",
        )
    return Response(
        content=upstream.content,
        status_code=upstream.status_code,
        headers={k: v for k, v in upstream.headers.items() if k.lower() not in ("transfer-encoding", "content-encoding")},
        media_type=upstream.headers.get("content-type"),
    )


@asynccontextmanager
async def _lifespan(_app: FastAPI) -> AsyncIterator[None]:
    os.environ["DEV_UNIFIED"] = "1"
    supervisor.load_env()
    supervisor.kill_legacy_dev_processes()

    loop = asyncio.get_running_loop()

    async def _supervise_loop() -> None:
        await loop.run_in_executor(None, lambda: supervisor.run_once(verbose=True))
        await loop.run_in_executor(None, lambda: supervisor.run_supervise(verbose=True))
        interval = float(os.environ.get("SUPERVISOR_INTERVAL", "15"))
        while True:
            await asyncio.sleep(interval)
            await loop.run_in_executor(None, lambda: supervisor.run_supervise(verbose=False))

    task = asyncio.create_task(_supervise_loop())
    try:
        # Accept dashboard/API traffic immediately; supervisor warms services in background.
        yield
    finally:
        task.cancel()
        try:
            await task
        except asyncio.CancelledError:
            pass


app = FastAPI(title="QuarkGate Dev Gateway", version="1.0.0", lifespan=_lifespan)


@app.get("/gateway/healthz")
def gateway_healthz() -> dict[str, str]:
    return {"status": "ok", "mode": "unified"}


@app.get("/gateway/routes")
def gateway_routes() -> JSONResponse:
    return JSONResponse(
        {
            "dashboard": "/",
            "quarkgate_api": "/qg/",
            "task_service": "/tasks/",
            "memory_curator": "/memory/",
            "execution": "/exec/",
            "hermes": "/hermes/",
            "note": "Single process — internal services on localhost, proxied here.",
        }
    )


_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"]


def _register_proxy(prefix: str, upstream: str) -> None:
    async def _proxy_path(path: str, request: Request) -> Response:
        return await _reverse_proxy(request, upstream, path)

    async def _proxy_root(request: Request) -> Response:
        return await _reverse_proxy(request, upstream, "")

    app.add_api_route(f"{prefix}/{{path:path}}", _proxy_path, methods=_METHODS)
    app.add_api_route(prefix, _proxy_root, methods=_METHODS)


for _prefix, _upstream in PROXY_PREFIXES:
    _register_proxy(_prefix, _upstream)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/api/probes")
async def api_probes() -> JSONResponse:
    results = await run_all_probes()
    counts = {"ok": 0, "degraded": 0, "down": 0, "skipped": 0}
    for r in results:
        counts[r["status"]] = counts.get(r["status"], 0) + 1
        r["health"] = normalize_health(r["status"])
    return JSONResponse({"summary": counts, "probes": results, "env": _env_summary()})


@app.post("/api/test-all")
async def api_test_all() -> JSONResponse:
    return JSONResponse(await run_api_test())


@app.get("/", response_class=HTMLResponse)
async def dashboard(request: Request) -> HTMLResponse:
    return templates.TemplateResponse(
        request,
        "dashboard.html",
        {
            "request": request,
            "summary": {"up": 0, "down": 0, "issue": 0, "skipped": 0},
            "probes": [],
            "env": _env_summary(),
            "tested_at": "— click Run API test",
        },
    )


def main() -> None:
    global _lock_handle
    supervisor.load_env()
    _lock_handle = supervisor.acquire_unified_lock()
    if _lock_handle is None:
        port = os.environ.get("DEV_GATEWAY_PORT", "8090")
        print(f"Dev gateway already running on :{port} — see $TMPDIR/quarkgate-dev/dev-unified.lock")
        raise SystemExit(0)

    import uvicorn

    port = int(os.environ.get("DEV_GATEWAY_PORT", "8090"))
    host = os.environ.get("DEV_GATEWAY_HOST", "127.0.0.1")
    print(f"QuarkGate dev gateway listening on http://{host}:{port}")
    print(f"  Dashboard:  http://{host}:{port}/")
    print(f"  QuarkGate:  http://{host}:{port}/qg/healthz")
    uvicorn.run(app, host=host, port=port, log_level="info")


if __name__ == "__main__":
    main()
