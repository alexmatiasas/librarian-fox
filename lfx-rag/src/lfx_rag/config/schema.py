"""
Configuration schema for lfx-rag.

Validated by Pydantic; YAML loaded by config.loader (see load_config for merge order).
"""

from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class DocumentSourceConfig(BaseModel):
    """Where to find documents.

    The loader tries the manifest first (produced by lfx), falling back to
    directory traversal if no manifest rows/documents are found. `library_dir`
    defaults to None because not every workflow uses a flat directory fallback.
    """

    manifest_path: Path | None = Field(
        default=Path("~/.local/share/lfx/manifest.db"),
        description="Path to lfx manifest SQLite database",
    )
    library_dir: Path | None = Field(
        default=None,
        description="Library directory fallback when manifest is unavailable",
    )


class FormatsConfig(BaseModel):
    """Allowed document formats."""

    pdf: bool = Field(default=True, description="Process PDF files")
    epub: bool = Field(default=True, description="Process EPUB files")


class SlidingWindowChunking(BaseModel):
    """Word/character-agnostic sliding window over tokenized text.

    With the default embedding model (e5-small) the hard limit is 512 tokens.
    chunk_size is kept under 512 to leave room for the "passage:" prefix added
    by rag.index(). overlap controls how much context is shared between
    adjacent chunks.
    """

    chunk_size: int = Field(default=500, ge=1, description="Max tokens per chunk")
    overlap: int = Field(default=100, ge=0, description="Overlap between consecutive chunks")


class SemanticChunking(BaseModel):
    """Reserved for future semantic chunking (not wired)."""

    enabled: bool = Field(default=False, description="Enable semantic chunking")
    threshold: float = Field(default=0.85, ge=0.0, le=1.0, description="Split threshold")
    min_chunk_size: int = Field(default=100, ge=1, description="Minimum chunk size in tokens")


class ChunkingConfig(BaseModel):
    """Chunking strategy and parameters."""

    model_config = ConfigDict(extra="forbid")

    default_strategy: Literal["sliding_window", "semantic"] = Field(
        default="sliding_window",
        description="Active chunking strategy",
    )
    sliding_window: SlidingWindowChunking = Field(default_factory=SlidingWindowChunking)
    semantic: SemanticChunking = Field(default_factory=SemanticChunking)


class LocalEmbeddings(BaseModel):
    """Local sentence-transformers embedding settings."""

    model: str = Field(
        default="intfloat/multilingual-e5-small",
        description="Sentence-transformers model identifier",
    )
    device: Literal["auto", "cpu", "mps", "cuda"] = Field(
        default="auto",
        description="Compute device (rag._device maps auto to None)",
    )
    batch_size: int = Field(default=32, ge=1, description="Embedding batch size")


class CloudEmbeddings(BaseModel):
    """Reserved (not wired)."""

    enabled: bool = Field(default=False, description="Unused")
    provider: Literal["google"] = Field(default="google", description="Unused")
    model: str = Field(default="text-embedding-004", description="Unused")


class EmbeddingsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid")

    local: LocalEmbeddings = Field(default_factory=LocalEmbeddings)
    cloud: CloudEmbeddings = Field(default_factory=CloudEmbeddings)


class VectorStoreConfig(BaseModel):
    """LanceDB settings. Vectors are stored normalized; rag uses LanceDB defaults."""

    model_config = ConfigDict(extra="forbid")

    uri: Path = Field(
        default=Path("~/.local/share/lfx-rag/lancedb"),
        description="LanceDB database URI",
    )
    table: str = Field(default="documents", description="LanceDB table name")
    search_limit: int = Field(default=5, ge=1, le=200, description="Top-k for retrieval")


class RerankerConfig(BaseModel):
    """Reserved (not wired)."""

    enabled: bool = Field(default=False, description="Unused")
    model: str = Field(default="bge-reranker-v2-m3", description="Unused")
    top_k: int = Field(default=5, ge=1, description="Unused")


class LocalGeneration(BaseModel):
    """Ollama settings (num_ctx/think are hardcoded in rag.answer())."""

    provider: Literal["ollama"] = Field(default="ollama", description="LLM provider")
    model: str = Field(default="qwen3:8b", description="Ollama model name")
    base_url: str = Field(default="http://localhost:11434", description="Ollama base URL")
    temperature: float = Field(default=0.2, ge=0.0, le=2.0, description="Sampling temperature")
    max_tokens: int = Field(default=2048, ge=1, description="Max tokens to generate")


class CloudGeneration(BaseModel):
    """Reserved (not wired)."""

    enabled: bool = Field(default=False, description="Unused")
    provider: Literal["groq"] = Field(default="groq", description="Unused")
    model: str = Field(default="llama-3.3-70b-versatile", description="Unused")
    temperature: float = Field(default=0.2, ge=0.0, le=2.0, description="Unused")
    max_tokens: int = Field(default=4096, ge=1, description="Unused")


class GenerationConfig(BaseModel):
    model_config = ConfigDict(extra="forbid")

    default: Literal["local", "cloud"] = Field(default="local", description="Active provider")
    local: LocalGeneration = Field(default_factory=LocalGeneration)
    cloud: CloudGeneration = Field(default_factory=CloudGeneration)


class MLflowConfig(BaseModel):
    """Reserved (not wired)."""

    enabled: bool = Field(default=False, description="Unused")
    tracking_uri: str | None = Field(default=None, description="Unused")
    experiment_name: str = Field(default="lfx-rag", description="Unused")


class MonitoringConfig(BaseModel):
    """Reserved (not wired)."""

    model_config = ConfigDict(extra="forbid")

    mlflow: MLflowConfig = Field(default_factory=MLflowConfig)


class StateConfig(BaseModel):
    """Reserved (not wired)."""

    model_config = ConfigDict(extra="forbid")

    db_path: Path = Field(
        default=Path("~/.local/share/lfx-rag/state.db"),
        description="Unused",
    )


class PipelineConfig(BaseModel):
    """Reserved (not wired)."""

    model_config = ConfigDict(extra="forbid")

    batch_size: int = Field(default=10, ge=1, description="Unused")
    max_concurrent: int = Field(default=4, ge=1, description="Unused")


class LfxRagConfig(BaseModel):
    model_config = ConfigDict(extra="forbid")

    document_source: DocumentSourceConfig = Field(default_factory=DocumentSourceConfig)
    formats: FormatsConfig = Field(default_factory=FormatsConfig)
    chunking: ChunkingConfig = Field(default_factory=ChunkingConfig)
    embeddings: EmbeddingsConfig = Field(default_factory=EmbeddingsConfig)
    vector_store: VectorStoreConfig = Field(default_factory=VectorStoreConfig)
    reranker: RerankerConfig = Field(default_factory=RerankerConfig)
    generation: GenerationConfig = Field(default_factory=GenerationConfig)
    monitoring: MonitoringConfig = Field(default_factory=MonitoringConfig)
    state: StateConfig = Field(default_factory=StateConfig)
    pipeline: PipelineConfig = Field(default_factory=PipelineConfig)
