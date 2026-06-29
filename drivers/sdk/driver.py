"""QuarkGate driver SDK helpers for Python contributors."""

from __future__ import annotations

import json
from typing import Any, Dict, Optional


def downstream(
    url: str,
    method: str,
    headers: Dict[str, str],
    body: Optional[Any],
    streaming: bool,
    provider: str,
    operation: str,
    estimate_micro: int,
) -> Dict[str, Any]:
    out: Dict[str, Any] = {
        "url": url,
        "method": method,
        "headers": headers,
        "streaming": streaming,
        "provider": provider,
        "operation": operation,
        "estimate_micro": estimate_micro,
    }
    if body is not None:
        out["body"] = body
    return out


def merge_usage(base: Dict[str, Any], patch: Dict[str, Any]) -> Dict[str, Any]:
    out = dict(base or {})
    out.update(patch or {})
    return out
