from __future__ import annotations

import logging
from dataclasses import dataclass, field
from typing import Any

logger = logging.getLogger(__name__)


@dataclass
class Chunk:
    doc_id: str
    text: str
    start_idx: int
    end_idx: int
    metadata: dict[str, Any] = field(default_factory=dict)


def _tokenize(text: str) -> list[str]:
    """Simple whitespace tokenization (word-level approximation)."""
    return text.split()


def _detokenize(tokens: list[str]) -> str:
    return " ".join(tokens)


def chunk_sliding_window(
    text: str,
    doc_id: str,
    chunk_size: int = 512,
    overlap: int = 128,
    metadata: dict[str, Any] | None = None,
) -> list[Chunk]:
    """Split text into overlapping chunks using a sliding window.

    Uses word-level tokenization as a simple approximation of
    token counting. Chunk boundaries fall on word boundaries.

    Args:
        text: Document text to chunk.
        doc_id: Document identifier for provenance.
        chunk_size: Number of tokens per chunk.
        overlap: Overlap between consecutive chunks (in tokens).
        metadata: Optional metadata to attach to each chunk.

    Returns:
        List of Chunk objects.
    """
    if not text:
        return []

    tokens = _tokenize(text)
    n = len(tokens)

    if n <= chunk_size:
        return [Chunk(doc_id=doc_id, text=text, start_idx=0, end_idx=n, metadata=metadata or {})]

    step = max(chunk_size - overlap, 1)
    meta = metadata or {}
    chunks: list[Chunk] = []
    start = 0

    while start < n:
        end = min(start + chunk_size, n)
        chunk_tokens = tokens[start:end]
        chunks.append(
            Chunk(
                doc_id=doc_id,
                text=_detokenize(chunk_tokens),
                start_idx=start,
                end_idx=end,
                metadata=meta,
            )
        )
        if end == n:
            break
        start += step

    return chunks
