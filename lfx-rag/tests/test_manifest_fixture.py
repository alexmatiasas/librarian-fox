from __future__ import annotations

import sqlite3
from pathlib import Path

import pymupdf

from lfx_rag.config.schema import LfxRagConfig
from lfx_rag.documents.loader import load_documents
from scripts.generate_testdata import generate_synthetic_testdata
from scripts.load_testdata import REPO_ROOT, build_fixture

FIXTURE_DB = REPO_ROOT / "lfx-rag" / "tests" / "fixtures" / "manifest.db"
TESTDATA_DIR = REPO_ROOT / "testdata"


def test_fixture_manifest_schema() -> None:
    conn = sqlite3.connect(str(FIXTURE_DB))
    try:
        tables = {
            row[0]
            for row in conn.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
        }
    finally:
        conn.close()
    assert {"documents", "classifications", "organization", "subject_groups"} <= tables


def test_fixture_manifest_populated() -> None:
    conn = sqlite3.connect(str(FIXTURE_DB))
    try:
        count = conn.execute("SELECT COUNT(*) FROM documents").fetchone()[0]
        by_type = dict(conn.execute("SELECT file_type, COUNT(*) FROM documents GROUP BY file_type"))
    finally:
        conn.close()
    assert count == 11
    assert set(by_type) == {"pdf", "epub", "djvu", "docx"}


def test_fixture_load_documents() -> None:
    cfg = LfxRagConfig()
    cfg.document_source.manifest_path = FIXTURE_DB
    cfg.document_source.library_dir = Path("/nonexistent")

    docs = load_documents(cfg)
    # 11 files - 1 djvu (unsupported) - 1 docx (unsupported) - 1 scan without
    # text layer (needs_ocr) = 8 loaded
    assert len(docs) == 8

    by_type = {d.metadata.get("tipo") for d in docs}
    assert {"libro", "paper", "tesis", "notas", "manual", "personal"} <= by_type

    assert not any(d.metadata.get("needs_ocr") for d in docs)
    assert not any(d.metadata.get("file_type") in ("djvu", "docx") for d in docs)

    paper = next(
        d for d in docs if d.metadata["titulo"].startswith("Some characteristic parameters")
    )
    assert paper.metadata["subject"] == "Physics"

    thesis = next(d for d in docs if d.metadata.get("tipo") == "tesis")
    assert thesis.metadata["language"] == "es"
    assert len(thesis.text) > 10_000


def test_fixture_rebuild_is_deterministic() -> None:
    first = build_fixture(FIXTURE_DB, TESTDATA_DIR)
    second = build_fixture(FIXTURE_DB, TESTDATA_DIR)
    assert first == second == 11


def test_generate_testdata_is_deterministic(tmp_path: Path) -> None:
    first = generate_synthetic_testdata(tmp_path / "a")
    second = generate_synthetic_testdata(tmp_path / "b")
    assert len(first) == len(second) == 9
    for fa, fb in zip(first, second, strict=True):
        assert fa.read_bytes() == fb.read_bytes(), f"mismatch: {fa.name}"


def test_generated_scan_pdf_has_no_text_layer(tmp_path: Path) -> None:
    generate_synthetic_testdata(tmp_path)
    scan = next(tmp_path.rglob("Scan sin texto.pdf"))
    assert scan is not None
    with pymupdf.open(str(scan)) as doc:
        for page in doc:
            text = page.get_text()
            assert isinstance(text, str) and text.strip() == ""
