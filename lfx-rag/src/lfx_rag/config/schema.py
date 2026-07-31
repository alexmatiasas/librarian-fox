from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field


class DocumentSourceConfig(BaseModel):
    manifest_path: Path = Field(
        default=Path("~/.local/share/lfx/manifest.db"),
        description="Path to lfx manifest SQLite database",
    )
    library_dir: Path = Field(
        default=Path("~/Biblioteca/"),
        description="Library directory (used as fallback if manifest unavailable)",
    )


class FormatsConfig(BaseModel):
    pdf: bool = Field(default=True, description="Process PDF files")
    epub: bool = Field(default=True, description="Process EPUB files")


class SlidingWindowChunking(BaseModel):
    chunk_size: int = Field(default=512, ge=1, description="Number of tokens per chunk")
    overlap: int = Field(default=128, ge=0, description="Overlap between consecutive chunks")
    unit: Literal["tokens", "chars"] = Field(default="tokens", description="Unit for chunk_size")


class SemanticChunking(BaseModel):
    enabled: bool = Field(default=False, description="Enable semantic chunking (experimental)")
    threshold: float = Field(
        default=0.85, ge=0.0, le=1.0, description="Similarity threshold for splitting"
    )
    min_chunk_size: int = Field(default=100, ge=1, description="Minimum chunk size in tokens")


class ChunkingConfig(BaseModel):
    default_strategy: Literal["sliding_window", "semantic"] = Field(
        default="sliding_window",
        description="Active chunking strategy",
    )
    sliding_window: SlidingWindowChunking = Field(default=SlidingWindowChunking())
    semantic: SemanticChunking = Field(default=SemanticChunking())


class LocalEmbeddings(BaseModel):
    model: str = Field(
        default="intfloat/multilingual-e5-small",
        description="Sentence-transformers model identifier",
    )
    dimension: int = Field(default=384, ge=1, description="Embedding vector dimension")
    device: Literal["auto", "cpu", "mps", "cuda"] = Field(
        default="auto",
        description="Compute device (auto detects mps > cuda > cpu)",
    )
    batch_size: int = Field(default=32, ge=1, description="Batch size for embedding generation")


class CloudEmbeddings(BaseModel):
    enabled: bool = Field(default=False, description="Enable cloud embeddings via API")
    provider: Literal["google"] = Field(default="google", description="Cloud embeddings provider")
    model: str = Field(default="text-embedding-004", description="Cloud embedding model name")


class EmbeddingsConfig(BaseModel):
    local: LocalEmbeddings = Field(default=LocalEmbeddings())
    cloud: CloudEmbeddings = Field(default=CloudEmbeddings())


class VectorStoreConfig(BaseModel):
    uri: Path = Field(
        default=Path("~/.local/share/lfx-rag/lancedb"),
        description="LanceDB database URI",
    )
    table: str = Field(default="documents", description="LanceDB table name")
    create_if_missing: bool = Field(
        default=True, description="Create the table if it doesn't exist"
    )
    search_limit: int = Field(
        default=20, ge=1, le=200, description="Default top-k for vector search"
    )
    search_metric: Literal["cosine", "l2", "dot"] = Field(
        default="cosine",
        description="Distance metric for vector similarity search",
    )


class RerankerConfig(BaseModel):
    enabled: bool = Field(default=False, description="Enable cross-encoder reranker")
    model: str = Field(
        default="bge-reranker-v2-m3",
        description="Reranker model identifier",
    )
    top_k: int = Field(default=5, ge=1, description="Number of results after reranking")


class LocalGeneration(BaseModel):
    provider: Literal["ollama"] = Field(default="ollama", description="Local LLM provider")
    model: str = Field(default="qwen3:8b", description="Local LLM model name")
    base_url: str = Field(
        default="http://localhost:11434",
        description="Ollama server base URL",
    )
    temperature: float = Field(default=0.2, ge=0.0, le=2.0, description="LLM temperature")
    max_tokens: int = Field(default=2048, ge=1, description="Maximum generation tokens")


class CloudGeneration(BaseModel):
    enabled: bool = Field(default=False, description="Enable cloud generation via API")
    provider: Literal["groq"] = Field(default="groq", description="Cloud LLM provider")
    model: str = Field(default="llama-3.3-70b-versatile", description="Cloud LLM model name")
    temperature: float = Field(default=0.2, ge=0.0, le=2.0, description="LLM temperature")
    max_tokens: int = Field(default=4096, ge=1, description="Maximum generation tokens")


class GenerationConfig(BaseModel):
    default: Literal["local", "cloud"] = Field(
        default="local", description="Active generation provider"
    )
    local: LocalGeneration = Field(default=LocalGeneration())
    cloud: CloudGeneration = Field(default=CloudGeneration())


class MLflowConfig(BaseModel):
    enabled: bool = Field(default=False, description="Enable MLflow experiment tracking")
    tracking_uri: str | None = Field(
        default=None,
        description="MLflow tracking URI (default: local mlruns/)",
    )
    experiment_name: str = Field(default="lfx-rag", description="MLflow experiment name")


class MonitoringConfig(BaseModel):
    mlflow: MLflowConfig = Field(default=MLflowConfig())


class StateConfig(BaseModel):
    db_path: Path = Field(
        default=Path("~/.local/share/lfx-rag/state.db"),
        description="Path to SQLite tracking database",
    )


class PipelineConfig(BaseModel):
    batch_size: int = Field(default=10, ge=1, description="Documents per batch")
    max_concurrent: int = Field(default=4, ge=1, description="Max parallel workers")


class LfxRagConfig(BaseModel):
    document_source: DocumentSourceConfig = Field(default=DocumentSourceConfig())
    formats: FormatsConfig = Field(default=FormatsConfig())
    chunking: ChunkingConfig = Field(default=ChunkingConfig())
    embeddings: EmbeddingsConfig = Field(default=EmbeddingsConfig())
    vector_store: VectorStoreConfig = Field(default=VectorStoreConfig())
    reranker: RerankerConfig = Field(default=RerankerConfig())
    generation: GenerationConfig = Field(default=GenerationConfig())
    monitoring: MonitoringConfig = Field(default=MonitoringConfig())
    state: StateConfig = Field(default=StateConfig())
    pipeline: PipelineConfig = Field(default=PipelineConfig())
