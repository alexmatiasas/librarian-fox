from __future__ import annotations

import logging
import sqlite3
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import pymupdf

from lfx_rag.config.schema import LfxRagConfig

logger = logging.getLogger(__name__)

SUPPORTED_EXTENSIONS: dict[str, str] = {
    ".pdf": "pdf",
    ".epub": "epub",
}


@dataclass
class Document:
    id: str
    path: Path
    text: str
    metadata: dict[str, Any] = field(default_factory=dict)


def load_documents(config: LfxRagConfig) -> list[Document]:
    """Load documents from the lfx manifest or library directory.

    Attempts manifest mode first (reads from lfx SQLite DB for metadata +
    file locations). Falls back to walking the library directory.
    """
    db_path = _resolve(config.document_source.manifest_path)
    if db_path and db_path.exists():
        logger.info("Loading from manifest: %s", db_path)
        docs = _load_from_manifest(config, db_path)
        if docs:
            return docs
        logger.info("Manifest yielded no documents, falling back to directory walk")

    lib_dir = _resolve(config.document_source.library_dir)
    if lib_dir and lib_dir.exists():
        logger.info("Loading from directory: %s", lib_dir)
        return _load_from_directory(config, lib_dir)

    logger.warning("No documents found — manifest and library dir both unavailable")
    return []


def _resolve(path: Path) -> Path:
    return path.expanduser().resolve()


def _load_from_manifest(config: LfxRagConfig, db_path: Path) -> list[Document]:
    conn = sqlite3.connect(str(db_path))
    conn.row_factory = sqlite3.Row
    cursor = conn.cursor()

    cursor.execute("PRAGMA foreign_keys = ON")

    rows = cursor.execute(
        """
        SELECT d.id,
               d.source_path,
               o.target_path,
               d.file_type,
               d.language,
               c.titulo,
               c.autor,
               c.subject,
               c.tipo,
               c.anio,
               c.editorial
        FROM documents d
        LEFT JOIN organization o ON d.id = o.document_id
        LEFT JOIN classifications c ON d.id = c.document_id AND c.accepted = TRUE
        ORDER BY d.created_at
        """
    ).fetchall()

    docs: list[Document] = []
    for row in rows:
        file_type = row["file_type"]
        if file_type not in ("pdf", "epub"):
            continue
        if not getattr(config.formats, file_type, True):
            logger.debug("Skipping %s (disabled in config)", file_type)
            continue

        file_path = _resolve(Path(row["target_path"] or row["source_path"]))
        if not file_path.exists():
            logger.warning("File not found: %s", file_path)
            continue

        text = _extract_text(file_path, file_type)
        if not text:
            logger.debug("No text extracted from %s", file_path)
            continue

        meta = _build_metadata(row)
        docs.append(Document(id=row["id"], path=file_path, text=text, metadata=meta))

    conn.close()
    logger.info("Loaded %d documents from manifest", len(docs))
    return docs


def _load_from_directory(config: LfxRagConfig, lib_dir: Path) -> list[Document]:
    docs: list[Document] = []
    extensions = _active_extensions(config)

    for path in lib_dir.rglob("*"):
        if not path.is_file() or path.suffix.lower() not in extensions:
            continue
        file_type = SUPPORTED_EXTENSIONS[path.suffix.lower()]

        text = _extract_text(path, file_type)
        if not text:
            continue

        doc_id = str(path.relative_to(lib_dir))
        meta = {"file_type": file_type, "source": "directory_fallback"}
        docs.append(Document(id=doc_id, path=path, text=text, metadata=meta))

    logger.info("Loaded %d documents from directory", len(docs))
    return docs


def _active_extensions(config: LfxRagConfig) -> set[str]:
    exts: set[str] = set()
    for ext, key in SUPPORTED_EXTENSIONS.items():
        if getattr(config.formats, key, True):
            exts.add(ext)
    return exts


def _extract_text(path: Path, file_type: str) -> str:
    try:
        if file_type in ("pdf", "epub"):
            doc = pymupdf.open(str(path))
            lines: list[str] = []
            for page in doc:
                page_text = page.get_text()
                if isinstance(page_text, str) and page_text:
                    lines.append(page_text)
            doc.close()
            return "".join(lines).strip()
        logger.warning("Unsupported file type: %s (%s)", file_type, path)
        return ""
    except Exception as e:
        logger.warning("Failed to extract text from %s: %s", path, e)
        return ""


def _build_metadata(row: sqlite3.Row) -> dict[str, Any]:
    meta: dict[str, Any] = {}
    for key in ("file_type", "language", "titulo", "autor", "subject", "tipo", "anio", "editorial"):
        val = row[key]
        if val is not None:
            meta[key] = val
    meta["source"] = "manifest"
    return meta
