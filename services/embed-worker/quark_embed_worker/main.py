from __future__ import annotations

import os
import sys
from pathlib import Path

# Allow editable install of quark-adapters from monorepo root.
_ROOT = Path(__file__).resolve().parents[4]
_ADAPTERS = _ROOT / "packages" / "provider-adapters"
if _ADAPTERS.is_dir() and str(_ADAPTERS) not in sys.path:
    sys.path.insert(0, str(_ADAPTERS))

from fastapi import FastAPI
from pydantic import BaseModel, Field

from quark_adapters.embeddings import get_embedding_adapter

app = FastAPI(title="quark-embed-worker", version="0.1.0")


class EmbedRequest(BaseModel):
    texts: list[str] = Field(min_length=1)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    provider = os.environ.get("EMBED_PROVIDER", "stub")
    return {"status": "ok", "provider": provider}


@app.post("/embed")
def embed(body: EmbedRequest) -> dict:
    adapter = get_embedding_adapter()
    vectors = adapter.embed(body.texts)
    return {
        "model": adapter.model_version,
        "dimensions": adapter.dimensions,
        "count": len(vectors),
        "embeddings": vectors,
    }


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("EMBED_WORKER_PORT", "8087"))
    uvicorn.run(app, host="127.0.0.1", port=port)
