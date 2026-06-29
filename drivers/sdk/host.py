"""QuarkGate driver host — JSON stdin/stdout IPC from Go edge (Python)."""
from __future__ import annotations

import importlib.util
import json
import sys
import traceback
from pathlib import Path
from typing import Any, Dict


def load_driver(provider: str) -> Any:
    driver_path = Path(__file__).resolve().parent.parent / provider / "driver.py"
    if not driver_path.exists():
        raise FileNotFoundError(f"driver not found for {provider}")
    spec = importlib.util.spec_from_file_location(f"qg_driver_{provider}", driver_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"cannot load {driver_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def fail(err: Exception, code: str = "DRIVER_ERROR") -> None:
    print(json.dumps({"error": str(err), "code": code}), file=sys.stderr)
    sys.exit(1)


def main() -> None:
    try:
        input_data: Dict[str, Any] = json.load(sys.stdin)
    except Exception as e:
        fail(e)

    provider = input_data.get("provider")
    if not provider:
        fail(ValueError("provider required"))

    driver = load_driver(provider)
    envelope = input_data.get("envelope")
    if isinstance(envelope, str):
        envelope = json.loads(envelope)

    action = input_data.get("action")
    try:
        if action == "estimate":
            micro = driver.estimate_max_cost(envelope or {"payload": {}})
            print(json.dumps({"estimate_micro": micro}))
            return
        if action == "prepare":
            downstream = driver.prepare_request(
                {
                    "envelope": envelope,
                    "credential": input_data.get("credential"),
                    "baseURL": input_data.get("baseURL"),
                }
            )
            print(json.dumps(downstream))
            return
        if action == "normalize":
            raw = input_data.get("raw_usage") or {}
            if hasattr(driver, "normalize_usage"):
                patched = driver.normalize_usage(raw, envelope)
                print(json.dumps({"raw_usage": patched}))
            else:
                print(json.dumps({"raw_usage": raw}))
            return
        if action == "parseResponse":
            if hasattr(driver, "parse_response"):
                raw = driver.parse_response(
                    {
                        "headers": input_data.get("headers") or {},
                        "body": input_data.get("body") or "",
                        "streaming": bool(input_data.get("streaming")),
                        "envelope": envelope,
                    }
                )
                print(json.dumps({"raw_usage": raw or {}}))
            else:
                print(json.dumps({"raw_usage": {}}))
            return
        if action == "healthCheck":
            if hasattr(driver, "health_check"):
                result = driver.health_check(
                    {
                        "baseURL": input_data.get("baseURL"),
                        "credential": input_data.get("credential"),
                    }
                )
                print(json.dumps(result))
            else:
                print(json.dumps({"ok": True, "latency_ms": 0, "message": "no health_check implemented"}))
            return
        if action == "poll":
            if not hasattr(driver, "poll_run"):
                fail(ValueError("poll not supported"))
            result = driver.poll_run(
                {
                    "baseURL": input_data.get("baseURL"),
                    "credential": input_data.get("credential"),
                    "poll_context": input_data.get("poll_context") or {},
                }
            )
            print(json.dumps(result))
            return
        fail(ValueError(f"unknown action: {action}"))
    except Exception as e:
        traceback.print_exc()
        fail(e)


if __name__ == "__main__":
    main()
