from __future__ import annotations

import os
from pathlib import Path

from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates

from quark_dev_dashboard.probes import normalize_health, run_all_probes, run_api_test

app = FastAPI(title="QuarkGate Dev Dashboard", version="0.2.0")

_TEMPLATES_DIR = Path(__file__).resolve().parent / "templates"
templates = Jinja2Templates(directory=str(_TEMPLATES_DIR))


def _env_summary() -> dict[str, str]:
    keys = [
        "QUARKGATE_URL",
        "MEMORY_CURATOR_URL",
        "ANYTYPE_API_URL",
        "EMBED_PROVIDER",
        "EXECUTOR_BACKEND",
        "EXECUTOR_MODE",
        "LETTA_AGENT_ID",
        "ZEP_MODE",
        "LANGSMITH_PROJECT",
        "LANGSMITH_SKILLS_DATASET",
        "DO_PROJECT_NAME",
        "APIFY_ACTOR_ID",
    ]
    return {k: os.environ.get(k, "—") for k in keys}


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
    """On-demand API health test — returns up / down / issue per service."""
    payload = await run_api_test()
    return JSONResponse(payload)


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


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("DEV_DASHBOARD_PORT", "8095"))
    uvicorn.run(app, host="127.0.0.1", port=port)
