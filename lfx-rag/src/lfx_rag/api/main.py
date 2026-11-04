from __future__ import annotations

import json

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import StreamingResponse

from lfx_rag.api.schemas import QueryRequest
from lfx_rag.config import load_config
from lfx_rag.rag import answer_stream

app = FastAPI(title="lfx-rag API", version="0.1.0")

# TODO: For production, change allow origins
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/query/stream")
def query_stream(req: QueryRequest) -> StreamingResponse:
    try:
        cfg = load_config()
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to load config: {e}") from e

    k = req.k or cfg.vector_store.search_limit

    def event_generator():
        try:
            for chunk in answer_stream(cfg, req.question, k):
                if chunk["type"] == "token":
                    # SSE format
                    payload = json.dumps({"token": chunk["content"]}, ensure_ascii=False)
                    yield f"data: {payload}\n\n"
                elif chunk["type"] == "done":
                    payload = json.dumps(
                        {"done": True, "sources": chunk["sources"]}, ensure_ascii=False
                    )
                    yield f"data: {payload}\n\n"
        except Exception as e:
            payload = json.dumps({"error": str(e)}, ensure_ascii=False)
            yield f"data: {payload}\n\n"

    return StreamingResponse(event_generator(), media_type="text/event-stream")
