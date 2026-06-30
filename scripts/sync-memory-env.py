#!/usr/bin/env python3
"""Enable memory flags when API keys are set; sync quarkgate keys to root .env."""
from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
QUARKGATE_ENV = ROOT / "quarkgate" / ".env"
QUARKOS_ENV = ROOT / ".env"

# When key is non-empty, set flag to value (both quarkgate and quarkOS).
KEY_PAIRS: list[tuple[str, str, str]] = [
    ("LETTA_API_KEY", "LETTA_ENABLED", "true"),
    ("ZEP_API_KEY", "ZEP_MODE", "cloud"),
    ("LANCEDB_API_KEY", "LANCE_ENABLED", "true"),
    ("HINDSIGHT_API_KEY", "HINDSIGHT_ENABLED", "true"),
    ("LANGSMITH_API_KEY", "LANGGRAPH_SKILLS_ENABLED", "true"),
    ("LANGSMITH_API_KEY", "LANGSMITH_TRACING", "true"),
    ("ANYTYPE_API_KEY", "ANYTYPE_ENABLED", "true"),
    ("OPENROUTER_API_KEY", "EMBED_PROVIDER", "openrouter"),
    ("E2B_API_KEY", "EXECUTOR_BACKEND", "e2b"),
    ("E2B_API_KEY", "DOCKER_ENABLED", "false"),
]

LIVE_PATHS = {
    "MEMORY_MATRIX_PATH": "config/memory_matrix.yaml",
    "MEMORY_SPECTRUM_CONFIG_PATH": "config/memory_spectrum.yaml",
    "EXECUTION_POLICY_PATH": "config/execution_policy.yaml",
    "CODING_AGENT_CONFIG_PATH": "config/coding_agent.yaml",
    "AUTONOMY_CONFIG_PATH": "config/autonomy.yaml",
}

SYNC_KEYS = [
    "LETTA_API_KEY",
    "LETTA_BASE_URL",
    "LETTA_AGENT_ID",
    "HINDSIGHT_API_KEY",
    "ZEP_API_KEY",
    "ZEP_API_BASE",
    "ZEP_USER_ID",
    "ZEP_THREAD_ID",
    "LANCEDB_API_KEY",
    "LANCEDB_URI",
    "LANCEDB_REGION",
    "KUZU_DB_PATH",
    "KUZU_BRIDGE_URL",
    "OPENROUTER_API_KEY",
    "LANGSMITH_API_KEY",
    "LANGSMITH_PROJECT",
    "LANGSMITH_PROJECT_ID",
    "LANGSMITH_ENDPOINT",
    "LANGSMITH_HUB_URL",
    "LANGSMITH_TRACING",
    "LANGSMITH_WORKSPACE_ID",
    "LANGSMITH_SKILLS_DATASET",
    "ANYTYPE_API_URL",
    "ANYTYPE_RECOVERY_PHRASE",
    "ANYTYPE_API_KEY",
    "ANYTYPE_BOT_NAME",
    "ANYTYPE_ACCOUNT_ID",
    "ANYTYPE_ACCOUNT_KEY",
    "SUPABASE_PROJECT_REF",
    "SUPABASE_PASSWORD",
    "APIFY_TOKEN",
    "APIFY_ACTOR_ID",
    "EMBED_PROVIDER",
    "EMBED_MODEL",
    "EMBED_DIMENSIONS",
    "DEV_DASHBOARD_URL",
    "DEV_GATEWAY_URL",
    "DEV_GATEWAY_PORT",
    "QUARKGATE_URL",
    "QUARKGATE_KEY",
    "QUARKGATE_ENABLED",
    "REDIS_URL",
    "DATABASE_URL",
    "DOCKER_ENABLED",
    "TASK_SERVICE_PORT",
    "TASK_API_URL",
    "E2B_API_KEY",
    "E2B_API_BASE",
    "E2B_TEMPLATE_ID",
    "EXECUTOR_BACKEND",
    "DIGITALOCEAN_TOKEN",
    "DO_REGION",
    "DO_PROJECT_NAME",
    "DO_PROJECT_ID",
    "DO_ACCOUNT_UUID",
    "DO_DROPLET_NAME",
    "DO_DROPLET_SIZE",
]

# Always-on when memory stack is configured (local services, no extra keys).
ALWAYS_ENABLE = {
    "KUZU_ENABLED": "true",
    "SLEEP_CYCLE_ENABLED": "true",
}

# Governance / not ready — never auto-enable.
NEVER_ENABLE = frozenset(
    {
        "PROMOTION_ENABLED",
        "CLIP_ENABLED",
        "BROWSER_ACTORS_ENABLED",
        "RSI_ENABLED",
        "A5_ENABLED",
    }
)


def parse_env(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        s = line.strip()
        if not s or s.startswith("#") or "=" not in s:
            continue
        key, _, val = s.partition("=")
        out[key.strip()] = val.strip()
    return out


def render_env(lines: list[str], updates: dict[str, str]) -> list[str]:
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
        elif key in ("ANYTYPE_ACCOUNT_KEY", "DO_API_TOKEN"):
            continue
        else:
            out.append(line)
    for key, val in updates.items():
        if key not in seen and val:
            out.append(f"{key}={val}")
    return out


def main() -> None:
    gate = parse_env(QUARKGATE_ENV.read_text(encoding="utf-8"))
    os_lines = QUARKOS_ENV.read_text(encoding="utf-8").splitlines()
    gate_lines = QUARKGATE_ENV.read_text(encoding="utf-8").splitlines()

    gate_updates: dict[str, str] = dict(ALWAYS_ENABLE)
    os_updates: dict[str, str] = dict(ALWAYS_ENABLE)

    for key, flag, value in KEY_PAIRS:
        if flag in NEVER_ENABLE:
            continue
        if gate.get(key):
            gate_updates[flag] = value
            os_updates[flag] = value

    for key in SYNC_KEYS:
        if gate.get(key):
            os_updates[key] = gate[key]

    for key, val in LIVE_PATHS.items():
        gate_updates[key] = val
        os_updates[key] = val

    if gate.get("OPENROUTER_API_KEY"):
        gate_updates["EMBED_PROVIDER"] = "openrouter"
        os_updates["EMBED_PROVIDER"] = "openrouter"

    if gate.get("E2B_API_KEY"):
        os_updates["EXECUTOR_MODE"] = "controller"
        os_updates["EXECUTOR_BACKEND"] = "e2b"
        os_updates["DOCKER_ENABLED"] = "false"
        gate_updates["EXECUTOR_BACKEND"] = "e2b"
        gate_updates["DOCKER_ENABLED"] = "false"

    if gate.get("QUARKGATE_KEY"):
        os_updates["QUARKGATE_KEY"] = gate["QUARKGATE_KEY"]
    if gate.get("QUARKGATE_URL"):
        os_updates["QUARKGATE_URL"] = gate["QUARKGATE_URL"]
    if gate.get("DEV_GATEWAY_URL"):
        os_updates["DEV_DASHBOARD_URL"] = gate["DEV_GATEWAY_URL"]
        os_updates["QUARKGATE_URL"] = gate.get("QUARKGATE_URL") or f"{gate['DEV_GATEWAY_URL'].rstrip('/')}/qg"
    gate_updates.setdefault("QUARKGATE_ENABLED", "true")
    os_updates.setdefault("QUARKGATE_ENABLED", "true")

    QUARKGATE_ENV.write_text("\n".join(render_env(gate_lines, gate_updates)) + "\n", encoding="utf-8")
    QUARKOS_ENV.write_text("\n".join(render_env(os_lines, os_updates)) + "\n", encoding="utf-8")
    print("Updated flags:", {k: gate_updates[k] for k in sorted(gate_updates)})
    print("Synced keys to quarkOS/.env:", [k for k in SYNC_KEYS if gate.get(k)])


if __name__ == "__main__":
    main()
