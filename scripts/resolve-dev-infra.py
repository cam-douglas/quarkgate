#!/usr/bin/env python3
"""Resolve dev infra without Docker — host Redis, Supabase Postgres, native Kùzu."""
from __future__ import annotations

import socket
import subprocess
import sys
from pathlib import Path
from urllib.parse import quote_plus

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT.parent
ENV_PATH = ROOT / ".env"


def _port_open(host: str, port: int, timeout: float = 0.4) -> bool:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def _parse_env(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        s = line.strip()
        if not s or s.startswith("#") or "=" not in s:
            continue
        key, _, val = s.partition("=")
        out[key.strip()] = val.strip().strip('"').strip("'")
    return out


def _render_env(lines: list[str], updates: dict[str, str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for line in lines:
        s = line.strip()
        if not s or s.startswith("#") or "=" not in s:
            out.append(line)
            continue
        key, _, _ = s.partition("=")
        key = key.strip()
        if key in updates:
            out.append(f"{key}={updates[key]}")
            seen.add(key)
        else:
            out.append(line)
    for key, val in updates.items():
        if key not in seen:
            out.append(f"{key}={val}")
    return out


def _project_ref(env: dict[str, str]) -> str:
    ref = env.get("SUPABASE_PROJECT_REF", "")
    if ref:
        return ref
    url = env.get("SUPABASE_URL", "")
    if "supabase.co" in url:
        return url.split("//", 1)[-1].split(".")[0]
    return ""


def _supabase_database_url(env: dict[str, str]) -> str | None:
    password = env.get("SUPABASE_PASSWORD") or env.get("SUPABASE_DB_PASSWORD", "")
    ref = _project_ref(env)
    if not password or not ref:
        return None
    user = env.get("SUPABASE_DB_USER", "postgres")
    host = env.get("SUPABASE_DB_HOST", f"db.{ref}.supabase.co")
    port = env.get("SUPABASE_DB_PORT", "5432")
    db = env.get("SUPABASE_DB_NAME", "postgres")
    return f"postgresql://{user}:{quote_plus(password)}@{host}:{port}/{db}?sslmode=require"


def resolve(env: dict[str, str]) -> dict[str, str]:
    """Return env updates for docker-free dev."""
    os_env = _parse_env((REPO / ".env").read_text(encoding="utf-8")) if (REPO / ".env").is_file() else {}
    merged = {**os_env, **env}

    updates: dict[str, str] = {
        "DOCKER_ENABLED": "false",
        "EXECUTOR_BACKEND": merged.get("EXECUTOR_BACKEND", "e2b"),
        "DEV_UNIFIED": "1",
        "DEV_GATEWAY_PORT": merged.get("DEV_GATEWAY_PORT", "8090"),
    }
    port = updates["DEV_GATEWAY_PORT"]
    gateway = f"http://127.0.0.1:{port}"
    updates["DEV_GATEWAY_URL"] = gateway
    updates["DEV_DASHBOARD_URL"] = gateway
    updates["DEV_DASHBOARD_PORT"] = port
    updates["QUARKGATE_URL"] = f"{gateway}/qg"
    updates["QUARKGATE_INTERNAL_URL"] = "http://127.0.0.1:8080"

    # Redis — prefer host brew (:6379), not Docker-mapped :6380
    if _port_open("127.0.0.1", 6379):
        updates["REDIS_URL"] = "redis://127.0.0.1:6379/0"
    elif _port_open("127.0.0.1", 6380):
        updates["REDIS_URL"] = "redis://127.0.0.1:6380/0"
    else:
        updates["REDIS_URL"] = merged.get("QUARKOS_REDIS_URL", "redis://127.0.0.1:6379/0")

    # Postgres — Supabase when local Docker PG (:5433) is down
    if _port_open("127.0.0.1", 5433):
        updates["DATABASE_URL"] = merged.get(
            "DATABASE_URL",
            "postgres://quarkgate:quarkgate@localhost:5433/quarkgate?sslmode=disable",
        )
    else:
        supa = _supabase_database_url(merged)
        if supa:
            updates["DATABASE_URL"] = supa
            if merged.get("SUPABASE_PASSWORD") and not env.get("SUPABASE_PASSWORD"):
                updates["SUPABASE_PASSWORD"] = merged["SUPABASE_PASSWORD"]
            ref = _project_ref(merged)
            if ref and not env.get("SUPABASE_PROJECT_REF"):
                updates["SUPABASE_PROJECT_REF"] = ref

    return updates


def main() -> int:
    if not ENV_PATH.is_file():
        print(f"Missing {ENV_PATH}", file=sys.stderr)
        return 1

    lines = ENV_PATH.read_text(encoding="utf-8").splitlines()
    env = _parse_env("\n".join(lines))
    updates = resolve(env)
    ENV_PATH.write_text("\n".join(_render_env(lines, updates)) + "\n", encoding="utf-8")

    print("Dev infra (no Docker):")
    for key in ("REDIS_URL", "DATABASE_URL", "QUARKGATE_URL", "DEV_GATEWAY_URL", "EXECUTOR_BACKEND"):
        val = updates.get(key, env.get(key, ""))
        if key == "DATABASE_URL" and val:
            val = val.split("@", 1)[-1]  # hide credentials in log
            val = f"***@{val}"
        print(f"  {key}={val}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
