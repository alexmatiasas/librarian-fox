"""Seed a lfx-compatible manifest SQLite DB from the testdata/ directory.

Usage:
    uv run python scripts/load_testdata.py [--db PATH] [--testdata DIR]

Reproduces the exact schema lfx produces (by executing
``internal/manifest/migrations/001_initial.sql``) and fills it with
documents from testdata/, inferring ``tipo``/``subject`` from the directory
layout (``<Tipo>/<Subject>/<file>``) and ``autor``/``titulo``/``anio``
from the filename (``Autor - Titulo (Ano).ext``).

The fixture DB is gitignored; run ``make seed`` to regenerate it after
adding or modifying files under testdata/.
"""

from __future__ import annotations

import argparse
import hashlib
import logging
import re
import sqlite3
from pathlib import Path
from typing import Any

import pymupdf

logger = logging.getLogger(__name__)

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parents[1]
TESTDATA_DIR = REPO_ROOT / "testdata"
MIGRATION_SQL = REPO_ROOT / "internal" / "manifest" / "migrations" / "001_initial.sql"
DEFAULT_DB = REPO_ROOT / "lfx-rag" / "tests" / "fixtures" / "manifest.db"

SUPPORTED_EXTENSIONS: dict[str, str] = {
    ".pdf": "pdf",
    ".epub": "epub",
    ".djvu": "djvu",
    ".docx": "docx",
}

TIPO_BY_FOLDER: dict[str, str] = {
    "Libros": "libro",
    "Papers": "paper",
    "Tesis": "tesis",
    "Notas": "notas",
    "Manuales": "manual",
    "Documentos Personales": "personal",
    "Reportes": "reporte",
    "Otros": "otro",
}

_PATTERN_AUTHOR_YEAR_TITLE = re.compile(
    r"^(?P<autor>.+?)\s+-\s+(?P<anio>\d{4})\s+-\s+(?P<titulo>.+)$"
)
_PATTERN_AUTHOR_TITLE_YEAR = re.compile(
    r"^(?P<autor>.+?)\s+-\s+(?P<titulo>.+?)\s+\((?P<anio>\d{4})\)$"
)

_STOPWORDS: dict[str, set[str]] = {
    "en": {"the", "and", "of", "to", "in", "a", "is", "that", "for", "on", "with", "as", "by"},
    "es": {"de", "la", "el", "y", "en", "que", "los", "las", "del", "una", "un", "para", "con"},
    "de": {"der", "die", "und", "in", "von", "den", "das", "mit", "ist", "zu", "auf", "für"},
}


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _detect_language(text: str) -> str | None:
    sample = " ".join(text.lower().split()[:500])
    if not sample:
        return None
    best: str | None = None
    best_score = 0
    for lang, words in _STOPWORDS.items():
        score = sum(word in words for word in sample.split())
        if score > best_score:
            best, best_score = lang, score
    return best


def _parse_filename(stem: str) -> dict[str, str | None]:
    for pattern in (_PATTERN_AUTHOR_YEAR_TITLE, _PATTERN_AUTHOR_TITLE_YEAR):
        match = pattern.match(stem)
        if match is not None:
            return {k: (v if v else None) for k, v in match.groupdict().items()}
    return {"autor": None, "titulo": stem.replace("_", " "), "anio": None}


def _extract(path: Path, file_type: str) -> tuple[str | None, int | None]:
    if file_type not in ("pdf", "epub"):
        return None, None
    doc = pymupdf.open(str(path))
    try:
        page_count = doc.page_count
        lines: list[str] = []
        for page in doc:
            text = page.get_text()
            if isinstance(text, str) and text:
                lines.append(text)
        return "".join(lines).strip() or None, page_count
    finally:
        doc.close()


def _infer_metadata(rel_path: Path) -> dict[str, str | None]:
    parts = rel_path.parts
    folder = parts[0] if parts else "Otros"
    subject = parts[1] if len(parts) > 2 else None
    return {
        "tipo": TIPO_BY_FOLDER.get(folder, "otro"),
        "subject": subject,
    }


def _build_row(path: Path, rel_path: Path) -> dict[str, Any]:
    file_type = SUPPORTED_EXTENSIONS[path.suffix.lower()]
    text, page_count = _extract(path, file_type)
    has_text_layer = text is not None
    parsed = _parse_filename(path.stem)
    inferred = _infer_metadata(rel_path)
    return {
        "id": _sha256(path),
        "source_path": str(path),
        "file_type": file_type,
        "file_size": path.stat().st_size,
        "page_count": page_count,
        "has_text_layer": has_text_layer,
        "needs_ocr": file_type == "pdf" and not has_text_layer,
        "language": _detect_language(text) if text else None,
        "tipo": inferred["tipo"],
        "subject": inferred["subject"],
        "autor": parsed["autor"],
        "titulo": parsed["titulo"],
        "anio": parsed["anio"],
    }


def _insert_document(conn: sqlite3.Connection, row: dict[str, Any]) -> None:
    conn.execute(
        """
        INSERT INTO documents
            (id, source_path, file_type, file_size, page_count,
             has_text_layer, needs_ocr, language)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            row["id"],
            row["source_path"],
            row["file_type"],
            row["file_size"],
            row["page_count"],
            row["has_text_layer"],
            row["needs_ocr"],
            row["language"],
        ),
    )
    conn.execute(
        """
        INSERT INTO classifications
            (document_id, source, model, tipo, tipo_original, subject,
             autor, titulo, anio, confidence, accepted, accepted_at, reason)
        VALUES (?, 'manual', 'testdata-seed', ?, ?, ?, ?, ?, ?, 1.0, TRUE,
                CURRENT_TIMESTAMP, 'inferred from testdata structure')
        """,
        (
            row["id"],
            row["tipo"],
            row["tipo"],
            row["subject"],
            row["autor"],
            row["titulo"],
            row["anio"],
        ),
    )
    conn.execute(
        """
        INSERT INTO organization (document_id, source_path, target_path)
        VALUES (?, ?, ?)
        """,
        (row["id"], row["source_path"], row["source_path"]),
    )


def _insert_subject_group(conn: sqlite3.Connection, subject: str) -> None:
    conn.execute(
        "INSERT OR IGNORE INTO subject_groups (name, name_en, name_es) VALUES (?, ?, ?)",
        (subject, subject, subject),
    )
    group_id = conn.execute("SELECT id FROM subject_groups WHERE name = ?", (subject,)).fetchone()[
        0
    ]
    conn.execute(
        "INSERT OR IGNORE INTO subject_aliases (alias, group_id, source) VALUES (?, ?, 'manual')",
        (subject, group_id),
    )


def _collect_files(testdata_dir: Path) -> list[Path]:
    return sorted(
        p
        for p in testdata_dir.rglob("*")
        if p.is_file() and p.suffix.lower() in SUPPORTED_EXTENSIONS
    )


def build_fixture(db_path: Path, testdata_dir: Path) -> int:
    """Create a lfx manifest fixture from a testdata directory.

    Returns the number of documents inserted.
    """
    migration = MIGRATION_SQL.read_text()

    if db_path.exists():
        db_path.unlink()
    for sidecar in (Path(str(db_path) + "-wal"), Path(str(db_path) + "-shm")):
        if sidecar.exists():
            sidecar.unlink()
    db_path.parent.mkdir(parents=True, exist_ok=True)

    conn = sqlite3.connect(str(db_path))
    try:
        conn.execute("PRAGMA foreign_keys = ON")
        conn.executescript(migration)

        subjects: set[str] = set()
        for path in _collect_files(testdata_dir):
            row = _build_row(path, path.relative_to(testdata_dir))
            _insert_document(conn, row)
            if row["subject"] is not None:
                subjects.add(row["subject"])

        for subject in sorted(subjects):
            _insert_subject_group(conn, subject)

        conn.commit()
    finally:
        conn.close()

    return len(_collect_files(testdata_dir))


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Seed a lfx manifest SQLite DB from the testdata/ directory."
    )
    parser.add_argument("--db", type=Path, default=DEFAULT_DB, help="Output manifest path")
    parser.add_argument(
        "--testdata", type=Path, default=TESTDATA_DIR, help="Input testdata directory"
    )
    parser.add_argument("--verbose", action="store_true", help="Enable debug logging")
    args = parser.parse_args()

    logging.basicConfig(level=logging.DEBUG if args.verbose else logging.INFO)
    count = build_fixture(args.db, args.testdata)
    logger.info("Wrote %d documents to %s", count, args.db)


if __name__ == "__main__":
    main()
