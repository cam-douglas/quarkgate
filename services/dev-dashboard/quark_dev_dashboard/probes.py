from __future__ import annotations

import asyncio
import os
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

import httpx


@dataclass
class ProbeResult:
    id: str
    name: str
    category: str
    status: str  # ok | degraded | down | skipped
    detail: str = ""
    latency_ms: float | None = None
    meta: dict[str, Any] = field(default_factory=dict)


async def _http_probe(
    client: httpx.AsyncClient,
    *,
    id: str,
    name: str,
    category: str,
    url: str,
    method: str = "GET",
    headers: dict[str, str] | None = None,
    json_body: dict | None = None,
    expect_status: int | tuple[int, ...] = 200,
    timeout: float = 8.0,
) -> ProbeResult:
    t0 = time.perf_counter()
    try:
        resp = await client.request(
            method, url, headers=headers, json=json_body, timeout=timeout
        )
        ms = (time.perf_counter() - t0) * 1000
        ok_codes = (expect_status,) if isinstance(expect_status, int) else expect_status
        if resp.status_code in ok_codes:
            return ProbeResult(id, name, category, "ok", f"HTTP {resp.status_code}", ms)
        return ProbeResult(
            id, name, category, "degraded", f"HTTP {resp.status_code}: {resp.text[:120]}", ms
        )
    except Exception as exc:  # noqa: BLE001
        ms = (time.perf_counter() - t0) * 1000
        return ProbeResult(id, name, category, "down", str(exc)[:200], ms)


async def _redis_ping(url: str) -> ProbeResult:
    t0 = time.perf_counter()
    try:
        import redis.asyncio as aioredis

        r = aioredis.from_url(url, socket_connect_timeout=3)
        pong = await r.ping()
        await r.aclose()
        ms = (time.perf_counter() - t0) * 1000
        if pong:
            return ProbeResult("redis-quarkgate", "Redis (QuarkGate :6380)", "infra", "ok", "PONG", ms)
        return ProbeResult("redis-quarkgate", "Redis (QuarkGate :6380)", "infra", "down", "no pong", ms)
    except Exception as exc:  # noqa: BLE001
        ms = (time.perf_counter() - t0) * 1000
        return ProbeResult("redis-quarkgate", "Redis (QuarkGate :6380)", "infra", "down", str(exc)[:200], ms)


async def _probe_execution_controller(
    client: httpx.AsyncClient,
    base_url: str,
) -> ProbeResult:
    """Health includes e2b backend status (ADR-035)."""
    url = base_url.rstrip("/") + "/health"
    result = await _http_probe(
        client,
        id="execution",
        name="Execution Controller (e2b)",
        category="execution",
        url=url,
    )
    if result.status != "ok":
        return result
    try:
        resp = await client.get(url, timeout=8.0)
        data = resp.json()
        backend = data.get("executor_backend", "")
        if backend != "e2b":
            return ProbeResult(
                "execution",
                "Execution Controller (e2b)",
                "execution",
                "issue",
                f"executor_backend={backend} (expected e2b)",
                result.latency_ms,
            )
        if data.get("e2b_configured") and not data.get("e2b_available"):
            return ProbeResult(
                "execution",
                "Execution Controller (e2b)",
                "execution",
                "issue",
                "e2b API key set but e2b_available=false",
                result.latency_ms,
            )
        detail = f"backend=e2b e2b_available={data.get('e2b_available')}"
        return ProbeResult(
            "execution",
            "Execution Controller (e2b)",
            "execution",
            "ok",
            detail,
            result.latency_ms,
        )
    except Exception as exc:  # noqa: BLE001
        return ProbeResult(
            "execution",
            "Execution Controller (e2b)",
            "execution",
            "issue",
            str(exc)[:200],
            result.latency_ms,
        )


async def _probe_local_service(
    client: httpx.AsyncClient,
    *,
    id: str,
    name: str,
    category: str,
    base_url: str,
    extra_headers: dict[str, str] | None = None,
) -> ProbeResult:
    """Try /health then /healthz — services use either convention."""
    headers = extra_headers or {}
    for suffix in ("/health", "/healthz"):
        url = base_url.rstrip("/") + suffix
        result = await _http_probe(
            client,
            id=id,
            name=name,
            category=category,
            url=url,
            headers=headers or None,
            expect_status=(200, 204),
        )
        if result.status == "ok":
            return result
    return result


async def run_all_probes() -> list[dict[str, Any]]:
    env = os.environ
    results: list[ProbeResult] = []
    root = Path(__file__).resolve().parents[3]
    repo_root = root.parent

    def _quarkgate_base() -> str:
        """Prefer direct internal gateway; fall back to unified /qg proxy."""
        internal = env.get("QUARKGATE_INTERNAL_URL", "http://127.0.0.1:8080").rstrip("/")
        if env.get("DEV_UNIFIED", "").lower() in ("1", "true", "yes"):
            proxy = env.get("QUARKGATE_URL", env.get("DEV_GATEWAY_URL", "http://127.0.0.1:8090") + "/qg").rstrip("/")
            return proxy
        return env.get("QUARKGATE_URL", internal).rstrip("/")

    qg_base = _quarkgate_base()

    async with httpx.AsyncClient(follow_redirects=True) as client:
        probe_tasks = [
            _http_probe(
                client,
                id="quarkgate",
                name="QuarkGate Gateway",
                category="core",
                url=qg_base + "/healthz",
            ),
            _http_probe(
                client,
                id="quarkgate-ready",
                name="QuarkGate Ready",
                category="core",
                url=qg_base + "/readyz",
            ),
            _probe_local_service(
                client,
                id="task-service",
                name="Task Service (MVQ)",
                category="core",
                base_url=env.get("TASK_API_URL", "http://127.0.0.1:8081"),
            ),
            _probe_local_service(
                client,
                id="memory-curator",
                name="Memory Curator",
                category="memory",
                base_url=env.get("MEMORY_CURATOR_URL", "http://127.0.0.1:8082"),
            ),
            _probe_local_service(
                client,
                id="embed-worker",
                name="Embed Worker",
                category="memory",
                base_url=env.get("EMBED_WORKER_URL", "http://127.0.0.1:8087"),
            ),
            _probe_local_service(
                client,
                id="letta-bridge",
                name="Letta Bridge",
                category="memory",
                base_url=env.get("LETTA_BRIDGE_URL", "http://127.0.0.1:8089"),
            ),
            _probe_local_service(
                client,
                id="zep-ingest",
                name="Zep Ingest Worker",
                category="memory",
                base_url=env.get("ZEP_INGEST_WORKER_URL", "http://127.0.0.1:8091"),
            ),
            _probe_local_service(
                client,
                id="sleep-worker",
                name="Sleep Worker",
                category="memory",
                base_url=env.get("MEMORY_SLEEP_WORKER_URL", "http://127.0.0.1:8088"),
            ),
            _probe_local_service(
                client,
                id="kuzu-bridge",
                name="Kùzu Bridge",
                category="memory",
                base_url=env.get("KUZU_BRIDGE_URL", "http://127.0.0.1:8093"),
            ),
            _probe_execution_controller(
                client,
                env.get("EXECUTION_CONTROLLER_URL", "http://127.0.0.1:8083"),
            ),
            _probe_local_service(
                client,
                id="hermes",
                name="Hermes",
                category="hermes",
                base_url=env.get("HERMES_URL", "http://127.0.0.1:8084"),
            ),
        ]

        anytype_key = env.get("ANYTYPE_API_KEY", "")
        if env.get("ANYTYPE_ENABLED", "").lower() in ("1", "true", "yes") and anytype_key:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="anytype",
                    name="Anytype API",
                    category="anytype",
                    url=env.get("ANYTYPE_API_URL", "http://127.0.0.1:31012").rstrip("/") + "/v1/spaces",
                    headers={
                        "Authorization": f"Bearer {anytype_key}",
                        "Anytype-Version": "2025-11-08",
                        "Accept": "application/json",
                    },
                    expect_status=(200, 204),
                )
            )
        else:
            results.append(
                ProbeResult("anytype", "Anytype API", "anytype", "skipped", "ANYTYPE not enabled or key missing")
            )

        # Cloud APIs
        or_key = env.get("OPENROUTER_API_KEY", "")
        if or_key:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="openrouter",
                    name="OpenRouter",
                    category="cloud",
                    url="https://openrouter.ai/api/v1/models",
                    headers={"Authorization": f"Bearer {or_key}"},
                    expect_status=200,
                )
            )
        else:
            results.append(ProbeResult("openrouter", "OpenRouter", "cloud", "skipped", "OPENROUTER_API_KEY unset"))

        letta_key = env.get("LETTA_API_KEY", "")
        letta_base = env.get("LETTA_BASE_URL", "https://api.letta.com").rstrip("/")
        if letta_key:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="letta-cloud",
                    name="Letta Cloud",
                    category="cloud",
                    url=f"{letta_base}/v1/agents/",
                    headers={"Authorization": f"Bearer {letta_key}"},
                    expect_status=200,
                )
            )

        zep_key = env.get("ZEP_API_KEY", "")
        zep_base = env.get("ZEP_API_BASE", "https://api.getzep.com").rstrip("/")
        zep_user = env.get("ZEP_USER_ID", "")
        if zep_key and env.get("ZEP_MODE") == "cloud":
            zep_url = f"{zep_base}/api/v2/users/{zep_user}" if zep_user else f"{zep_base}/api/v2/users-ids"
            probe_tasks.append(
                _http_probe(
                    client,
                    id="zep",
                    name="Zep Cloud",
                    category="cloud",
                    url=zep_url,
                    headers={"Authorization": f"Api-Key {zep_key}"},
                    expect_status=(200, 404),
                )
            )

        ls_key = env.get("LANGSMITH_API_KEY", "")
        ls_endpoint = env.get("LANGSMITH_ENDPOINT", "https://api.smith.langchain.com").rstrip("/")
        ls_project = env.get("LANGSMITH_PROJECT", "")
        if ls_key and ls_project:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="langsmith",
                    name="LangSmith (APAC)",
                    category="cloud",
                    url=f"{ls_endpoint}/api/v1/sessions",
                    headers={"x-api-key": ls_key},
                    expect_status=(200, 401, 403),
                )
            )

        e2b_key = env.get("E2B_API_KEY", "")
        if e2b_key:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="e2b",
                    name="E2B Sandboxes",
                    category="execution",
                    url=f"{env.get('E2B_API_BASE', 'https://api.e2b.dev').rstrip('/')}/sandboxes",
                    headers={"X-API-Key": e2b_key},
                    expect_status=(200, 404),
                )
            )

        do_token = env.get("DIGITALOCEAN_TOKEN", "")
        if do_token:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="digitalocean",
                    name="DigitalOcean",
                    category="cloud",
                    url="https://api.digitalocean.com/v2/account",
                    headers={"Authorization": f"Bearer {do_token}"},
                    expect_status=200,
                )
            )

        apify = env.get("APIFY_TOKEN", "")
        if apify:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="apify",
                    name="Apify",
                    category="cloud",
                    url="https://api.apify.com/v2/acts",
                    headers={"Authorization": f"Bearer {apify}"},
                    expect_status=200,
                )
            )

        sb_url = env.get("SUPABASE_URL", "")
        sb_key = env.get("SUPABASE_ANON_KEY", "")
        if sb_url and sb_key:
            probe_tasks.append(
                _http_probe(
                    client,
                    id="supabase",
                    name="Supabase",
                    category="cloud",
                    url=f"{sb_url.rstrip('/')}/rest/v1/",
                    headers={"apikey": sb_key, "Authorization": f"Bearer {sb_key}"},
                    expect_status=(200, 401),
                )
            )

        hs_key = env.get("HINDSIGHT_API_KEY", "")
        hs_path = env.get("HINDSIGHT_DATA_PATH", "data/agents.mmd/tier2")
        if hs_key:
            hs_full = Path(hs_path)
            if not hs_full.is_absolute():
                hs_full = repo_root / hs_path
            results.append(
                ProbeResult(
                    "hindsight",
                    "Hindsight (git tier2)",
                    "memory",
                    "ok" if hs_full.exists() else "degraded",
                    f"key set; data at {hs_full}",
                    meta={"path_exists": hs_full.exists()},
                )
            )

        gathered = await asyncio.gather(*probe_tasks, return_exceptions=True)
        for item in gathered:
            if isinstance(item, ProbeResult):
                results.append(item)
            elif isinstance(item, Exception):
                results.append(ProbeResult("unknown", "Unknown", "core", "down", str(item)[:200]))

    # Redis — QuarkGate ledger uses REDIS_URL; memory stack may use QUARKOS_REDIS_URL
    redis_url = env.get("REDIS_URL") or env.get("QUARKOS_REDIS_URL", "redis://127.0.0.1:6379/0")
    try:
        import redis.asyncio as aioredis  # noqa: F401

        results.append(await _redis_ping(redis_url))
    except ImportError:
        results.append(
            ProbeResult(
                "redis-quarkgate",
                "Redis (QuarkGate)",
                "infra",
                "skipped",
                "install redis package for live ping",
            )
        )
    except Exception as exc:  # noqa: BLE001
        results.append(
            ProbeResult("redis-quarkgate", "Redis (QuarkGate)", "infra", "down", str(exc)[:200])
        )

    # Filesystem stores
    kuzu_path = Path(env.get("KUZU_DB_PATH", "data/kuzu/memory_graph.kuzu"))
    lance_path = Path(env.get("LANCEDB_URI", "data/lancedb"))
    kuzu_full = root / kuzu_path if not kuzu_path.is_absolute() else kuzu_path
    lance_full = root / lance_path if not lance_path.is_absolute() else lance_path
    results.append(
        ProbeResult(
            "kuzu-fs",
            "Kùzu DB (local)",
            "memory",
            "ok" if kuzu_full.exists() else "degraded",
            str(kuzu_full),
            meta={"exists": kuzu_full.exists()},
        )
    )
    results.append(
        ProbeResult(
            "lance-fs",
            "LanceDB (local)",
            "memory",
            "ok" if lance_full.exists() else "degraded",
            str(lance_full),
            meta={"exists": lance_full.exists()},
        )
    )

    # Config files
    for key, label in (
        ("MEMORY_SPECTRUM_CONFIG_PATH", "memory_spectrum.yaml"),
        ("MEMORY_MATRIX_PATH", "memory_matrix.yaml"),
        ("EXECUTION_POLICY_PATH", "execution_policy.yaml"),
    ):
        rel = env.get(key, "")
        if rel:
            p = root / rel
            if not p.exists() and rel.startswith("config/"):
                p = repo_root / rel
            results.append(
                ProbeResult(
                    f"cfg-{label}",
                    f"Config {label}",
                    "config",
                    "ok" if p.exists() else "down",
                    str(p),
                )
            )

    return [asdict(r) for r in results]


def normalize_health(status: str) -> str:
    """Map probe status to dashboard health: up | down | issue | skipped."""
    if status == "ok":
        return "up"
    if status == "degraded":
        return "issue"
    if status == "down":
        return "down"
    return "skipped"


async def run_api_test() -> dict:
    """Run all probes and return simplified up/down/issue report."""
    import datetime as dt

    probes = await run_all_probes()
    summary = {"up": 0, "down": 0, "issue": 0, "skipped": 0}
    for row in probes:
        health = normalize_health(row["status"])
        row["health"] = health
        summary[health] = summary.get(health, 0) + 1
    return {
        "tested_at": dt.datetime.now(dt.UTC).isoformat(),
        "summary": summary,
        "probes": probes,
        "env": {
            k: os.environ.get(k, "—")
            for k in (
                "QUARKGATE_URL",
                "MEMORY_CURATOR_URL",
                "ANYTYPE_API_URL",
                "TASK_API_URL",
                "HERMES_URL",
            )
        },
    }
