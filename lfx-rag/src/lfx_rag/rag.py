from __future__ import annotations

import json
import logging
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import httpx
import lancedb
from sentence_transformers import SentenceTransformer

from lfx_rag.config.schema import LfxRagConfig
from lfx_rag.documents.chunker import chunk_sliding_window
from lfx_rag.documents.loader import load_documents

logger = logging.getLogger(__name__)

PROMPT = """
Answer the question using only the numbered context passages.
Cite passages as [1], [2]. If the answer is not in the context, say so.
Answer in the language of the question.

{context}

Question: {question}
Answer:
"""


def _device(cfg: LfxRagConfig) -> str | None:
    """Type of embeddings

    available: auto | cpu | mps | cuda

    """
    device = cfg.embeddings.local.device
    return None if device == "auto" else device


def _embedder(cfg: LfxRagConfig) -> SentenceTransformer:
    return SentenceTransformer(cfg.embeddings.local.model, device=_device(cfg))


def _table(cfg: LfxRagConfig) -> Any:
    uri = Path(cfg.vector_store.uri).expanduser()
    db = lancedb.connect(str(uri))
    return db.open_table(cfg.vector_store.table)


def index(cfg: LfxRagConfig) -> tuple[int, int]:
    """Embed all manifest documents and write them to LanceDB. Returns (docs, chunks)."""
    docs = load_documents(cfg)
    if not docs:
        return 0, 0

    window = cfg.chunking.sliding_window
    model = _embedder(cfg)
    rows: list[dict[str, Any]] = []
    chunk_count = 0
    for doc in docs:
        chunks = chunk_sliding_window(
            doc.text,
            doc.id,
            chunk_size=window.chunk_size,
            overlap=window.overlap,
            metadata=doc.metadata,
            tokenizer=model.tokenizer,
        )
        chunk_count += len(chunks)
        for i, chunk in enumerate(chunks):
            # TODO: Maybe we should improve this call to avoid hardcoded fields
            rows.append(
                {
                    "chunk_id": f"{doc.id}:{i}",
                    "doc_id": doc.id,
                    "chunk_index": i,
                    "text": chunk.text,
                    "source_path": str(doc.path),
                    "title": doc.metadata.get("titulo", ""),
                    "author": doc.metadata.get("autor", ""),
                    "year": doc.metadata.get("anio", ""),
                    "language": doc.metadata.get("language", ""),
                    "tipo": doc.metadata.get("tipo", ""),
                    "subject": doc.metadata.get("subject", ""),
                }
            )

    vectors = model.encode(
        [f"passage: {row['text']}" for row in rows],
        batch_size=cfg.embeddings.local.batch_size,
        normalize_embeddings=True,
        show_progress_bar=True,
    )
    for row, vector in zip(rows, vectors, strict=True):
        row["vector"] = vector.tolist()

    uri = Path(cfg.vector_store.uri).expanduser()
    db = lancedb.connect(str(uri))
    # shortcut: full overwrite on every run, switch to per-doc upsert when reindexing is routine
    db.create_table(cfg.vector_store.table, data=rows, mode="overwrite")
    logger.info("Indexed %d chunks from %d documents", chunk_count, len(docs))
    return len(docs), chunk_count


def retrieve(cfg: LfxRagConfig, question: str, k: int) -> list[dict[str, Any]]:
    model = _embedder(cfg)
    query = model.encode([f"query: {question}"], normalize_embeddings=True)[0]
    hits = _table(cfg).search(query.tolist()).limit(k).to_list()
    for hit in hits:
        hit.pop("vector", None)
    return hits


def answer(cfg: LfxRagConfig, question: str, k: int) -> tuple[str, list[dict[str, Any]]]:
    hits = retrieve(cfg, question, k)
    context = "\n\n".join(
        f"[{i}] {hit['title'] or Path(hit['source_path']).name} — {hit['text']}"
        for i, hit in enumerate(hits, start=1)
    )
    gen = cfg.generation.local
    payload = {
        "model": gen.model,
        "prompt": PROMPT.format(context=context, question=question),
        "stream": False,
        "think": False,
        "options": {
            "temperature": gen.temperature,
            "num_predict": gen.max_tokens,
            "num_ctx": 8192,
        },
    }
    response = httpx.post(f"{gen.base_url}/api/generate", json=payload, timeout=300)
    response.raise_for_status()
    return response.json()["response"].strip(), hits


def answer_stream(cfg: LfxRagConfig, question: str, k: int) -> Iterator[dict[str, Any]]:
    """Stream answer from Ollama token by token via SSE-friendly dicts."""
    hits = retrieve(cfg, question, k)
    context = "\n\n".join(
        f"[{i}] {hit['title'] or Path(hit['source_path']).name} — {hit['text']}"
        for i, hit in enumerate(hits, start=1)
    )
    prompt = PROMPT.format(context=context, question=question)

    gen = cfg.generation.local
    payload = {
        "model": gen.model,
        "prompt": prompt,
        "stream": True,
        "think": False,
        "options": {
            "temperature": gen.temperature,
            "num_predict": gen.max_tokens,
            "num_ctx": 8192,
        },
    }

    url = f"{gen.base_url}/api/generate"
    with httpx.stream("POST", url, json=payload, timeout=None) as resp:
        resp.raise_for_status()
        for line in resp.iter_lines():
            if not line:
                continue
            try:
                data = json.loads(line)
            except json.JSONDecodeError:
                continue
            if data.get("response"):
                yield {"type": "token", "content": data["response"]}
            if data.get("done"):
                # Send sources at the end
                sources = []
                for i, hit in enumerate(hits, start=1):
                    sources.append(
                        {
                            "index": i,
                            "title": hit.get("title") or Path(hit["source_path"]).name,
                            "subject": hit.get("subject"),
                            "source_path": hit.get("source_path"),
                            "doc_id": hit.get("doc_id"),
                        }
                    )
                yield {"type": "done", "sources": sources}
                break
