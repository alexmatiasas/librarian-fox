import marimo

__generated_with = "0.23.15"
app = marimo.App(width="medium")


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    # lfx-rag — EDA del pipeline RAG

    Flujo: **config → loader → chunker**, inspeccionando el manifest de lfx.

    - **0** — Toggle `fixtures/real` + conexión SQLite read-only
    - **1** — Manifest: tablas + row counts (shadow FTS excluido)
    - **2** — Manifest: `file_type` en la DB vs indexables por el loader
    - **3** — Manifest: JOIN `documents ⋈ classifications ⋈ organization`
    - **4** — Manifest: diccionarios `subject_groups` + `tags`
    - **5** — Manifest: ficha de la tesis (documento grande real)
    - **6** — Config: `load_config()` recortado
    - **7** — `load_documents()`: qué carga y qué excluye
    - **8** — `chunk_sliding_window()` sobre un paper real
    - **9** — Resumen: pipeline completo sobre la librería
    - **10** — (opcional) scale check con PDF grande

    **Manifest**: base SQLite del CLI `lfx` (Go). Se inspecciona en **solo lectura**
    (`mode=ro`) para garantizar que el EDA nunca modifica datos.
    """)
    return


@app.cell
def _():
    import sqlite3
    from pathlib import Path

    import marimo as mo

    from lfx_rag.config import load_config
    from lfx_rag.documents import Document
    from lfx_rag.documents.chunker import chunk_sliding_window
    from lfx_rag.documents.loader import SUPPORTED_EXTENSIONS, load_documents

    return (
        Document,
        Path,
        SUPPORTED_EXTENSIONS,
        chunk_sliding_window,
        load_config,
        load_documents,
        mo,
        sqlite3,
    )


@app.cell
def _(mo):
    use_fixtures = mo.ui.switch(
        value=True,
        label="Usar `tests/fixtures/manifest.db` (testdata sintético) en vez del manifest real",
    )
    use_fixtures
    return (use_fixtures,)


@app.cell
def _(Path, use_fixtures):
    db_path = (
        Path("tests/fixtures/manifest.db")
        if use_fixtures.value
        else Path("~/.local/share/lfx/manifest.db").expanduser()
    )
    return (db_path,)


@app.cell
def _(Path, sqlite3):
    def make_ro_conn(path: Path) -> sqlite3.Connection:
        conn = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
        conn.row_factory = sqlite3.Row
        return conn

    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Manifest — tablas y conteos (solo lectura)
    """)
    return


@app.cell
def _(db_path, mo):
    def _eda_tables():
        if not db_path.exists():
            return mo.md(f"⚠️ `{db_path}` no existe — corre `make seed` o activa `fixtures`.")

        conn = make_ro_conn(db_path)
        tables = [
            row[0]
            for row in conn.execute(
                "SELECT name FROM sqlite_master "
                "WHERE type='table' AND name NOT LIKE 'classifications_fts%' "
                "ORDER BY name"
            )
        ]
        counts = [(t, conn.execute(f"SELECT count(*) FROM {t}").fetchone()[0]) for t in tables]
        conn.close()

        purposes = {
            "_migrations": "Version tracking de migraciones",
            "documents": "Identidad de cada archivo (SHA-256 content-addressed)",
            "known_types": "Tipos válidos para fuzzy matching",
            "subject_groups": "Agrupación de subjects emergentes",
            "subject_aliases": "Alias → grupo (ej: ML → Computer Science)",
            "classifications": "Historial de intentos de clasificación",
            "classifications_fts": "FTS5 (external content) — espejo de classifications",
            "tags": "Etiquetas multi-idioma",
            "document_tags": "Vínculo muchos-a-muchos doc ↔ tag",
            "organization": "Dónde quedó cada documento tras lfx apply",
            "duplicates": "Vínculo entre documentos duplicados",
        }
        return mo.ui.table(
            [{"tabla": t, "filas": n, "propósito": purposes.get(t, "")} for t, n in counts],
            label="Tablas del manifest",
        )

    _eda_tables()
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Manifest — `file_type`: la DB vs lo indexable
    """)
    return


@app.cell
def _(SUPPORTED_EXTENSIONS, db_path, mo):
    def _eda_filetypes():
        if not db_path.exists():
            return mo.md(f"⚠️ `{db_path}` no existe.")

        conn = make_ro_conn(db_path)
        db_counts = dict(
            conn.execute("SELECT file_type, count(*) FROM documents GROUP BY file_type")
        )
        conn.close()

        rows = []
        for ft in sorted(set(db_counts) | set(SUPPORTED_EXTENSIONS.values())):
            rows.append(
                {
                    "file_type": ft,
                    "en DB": db_counts.get(ft, 0),
                    "indexable por loader": ft in SUPPORTED_EXTENSIONS.values(),
                }
            )
        return mo.ui.table(
            rows,
            label=(
                "Distribución por tipo de archivo: realidad en la DB vs "
                "formatos que el loader soporta"
            ),
        )

    _eda_filetypes()
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Manifest — fichas JOIN `documents ⋈ classifications ⋈ organization`
    """)
    return


@app.cell
def _(Path, db_path, mo):
    def _eda_joins():
        if not db_path.exists():
            return mo.md(f"⚠️ `{db_path}` no existe.")

        conn = make_ro_conn(db_path)
        rows = conn.execute(
            """
            SELECT d.file_type,
                   c.tipo,
                   c.subject,
                   c.titulo,
                   c.autor,
                   c.anio,
                   c.confidence,
                   d.page_count,
                   d.needs_ocr,
                   o.target_path
            FROM documents d
            LEFT JOIN classifications c
                   ON d.id = c.document_id AND c.accepted = TRUE
            LEFT JOIN organization o ON d.id = o.document_id
            ORDER BY d.created_at
            """
        ).fetchall()
        conn.close()

        return mo.ui.table(
            [
                {
                    "destino": (Path(r["target_path"]).name if r["target_path"] else "—")[:55],
                    "tipo": r["tipo"],
                    "subject": r["subject"],
                    "título": r["titulo"],
                    "autor": r["autor"],
                    "año": r["anio"],
                    "págs": r["page_count"],
                    "needs_ocr": bool(r["needs_ocr"]),
                    "conf": r["confidence"],
                }
                for r in rows
            ],
            label="Documentos con clasificación aceptada y ubicación organizada",
        )

    _eda_joins()
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Manifest — diccionarios `subject_groups` y `tags`
    """)
    return


@app.cell
def _(db_path, mo):
    def _eda_dicts():
        if not db_path.exists():
            return mo.md(f"⚠️ `{db_path}` no existe.")

        conn = make_ro_conn(db_path)
        groups = conn.execute(
            "SELECT name, name_en, name_es FROM subject_groups ORDER BY id"
        ).fetchall()
        tags = conn.execute("SELECT key, label_en, label_es FROM tags ORDER BY id").fetchall()
        conn.close()

        return mo.vstack(
            [
                mo.ui.table(
                    [dict(row) for row in groups],
                    label="subject_groups (subjects emergentes agrupados)",
                ),
                mo.ui.table([dict(row) for row in tags], label="tags (multi-idioma)"),
            ]
        )

    _eda_dicts()
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Manifest — ficha de la tesis (documento grande real)
    """)
    return


@app.cell
def _(db_path, mo):
    def _eda_tesis():
        if not db_path.exists():
            return mo.md(f"⚠️ `{db_path}` no existe.")

        conn = make_ro_conn(db_path)
        row = conn.execute(
            """
            SELECT d.file_type, d.file_size, d.page_count, d.language,
                   d.has_text_layer, c.titulo, c.autor, c.anio, c.subject, c.tipo
            FROM documents d
            LEFT JOIN classifications c
                   ON d.id = c.document_id AND c.accepted = TRUE
            WHERE c.tipo = 'tesis'
            """
        ).fetchone()
        conn.close()

        if row is None:
            return mo.md("Sin tesis en la DB seleccionada.")

        mb = row["file_size"] / (1024 * 1024)
        return mo.md(
            f"**{row['titulo']}** — {row['autor']}, {row['anio']}\n\n"
            f"- **formato**: `{row['file_type']}` · **idioma**: `{row['language']}`\n"
            f"- **tamaño**: `{mb:.1f} MB` · **páginas**: `{row['page_count']}`\n"
            f"- **capa de texto**: `{bool(row['has_text_layer'])}` (no necesita OCR)\n"
            f"- **subject**: `{row['subject']}` · **tipo**: `{row['tipo']}`"
        )

    _eda_tesis()
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Config — `load_config()` recortado
    """)
    return


@app.cell
def _(load_config, mo):
    loaded = load_config()
    src = loaded.document_source
    mo.vstack(
        [
            mo.md(
                f"- **manifest_path**: `{src.manifest_path}`  \n"
                f"- **library_dir**: `{src.library_dir}` (fallback)"
            ),
            mo.md(
                f"- **formatos activos**: "
                f"`pdf={loaded.formats.pdf}`, `epub={loaded.formats.epub}`  \n"
                f"(`.djvu`/`.docx`/`.doc` están en la DB pero `SUPPORTED_EXTENSIONS` "
                f"del loader solo indexa pdf/epub)"
            ),
            mo.md(
                f"- **chunking**: `{loaded.chunking.default_strategy}` "
                f"(chunk_size={loaded.chunking.sliding_window.chunk_size}, "
                f"overlap={loaded.chunking.sliding_window.overlap}, "
                f"unit={loaded.chunking.sliding_window.unit})"
            ),
            mo.md(
                f"- **embeddings local**: `{loaded.embeddings.local.model}` "
                f"(dim={loaded.embeddings.local.dimension}, "
                f"device={loaded.embeddings.local.device})"
            ),
            mo.md(
                f"- **vector store**: `{loaded.vector_store.uri}` · "
                f"`{loaded.vector_store.search_metric}` top-{loaded.vector_store.search_limit}"
            ),
            mo.md(
                f"- **generation**: `{loaded.generation.default}` "
                f"(`{loaded.generation.local.model}`)"
            ),
        ]
    )
    return (loaded,)


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Loader — `load_documents()`: qué carga y qué excluye
    """)
    return


@app.cell
def _(Document, Path, load_documents, loaded, mo, use_fixtures):
    test_cfg = loaded.model_copy(deep=True)
    if use_fixtures.value:
        test_cfg.document_source.manifest_path = Path("tests/fixtures/manifest.db").resolve()

    docs: list[Document] = load_documents(test_cfg)

    mo.vstack(
        [
            mo.md(
                f"**{len(docs)} documentos cargados** desde "
                f"`{test_cfg.document_source.manifest_path.resolve()}`"
            ),
            mo.md(
                "Excluidos por el loader (11 en la DB → 8):  \n"
                "- **2** sin soporte en `SUPPORTED_EXTENSIONS`: `.djvu`, `.docx`  \n"
                "- **1** sin capa de texto extraíble: `Scan sin texto.pdf` → deriva a la fase OCR"
            ),
            mo.ui.table(
                [
                    {
                        "id": d.id[:8],
                        "archivo": d.path.name[:55],
                        "tipo": d.metadata.get("file_type"),
                        "idioma": d.metadata.get("language", "—"),
                        "chars": len(d.text),
                        "≈tokens": len(d.text.split()),
                    }
                    for d in sorted(docs, key=lambda x: x.path.name)
                ],
                label="Documentos indexables",
            ),
        ]
    )
    return (docs,)


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Chunker — `chunk_sliding_window()` sobre un paper real
    """)
    return


@app.cell
def _(chunk_sliding_window, docs: "list[Document]", loaded, mo):
    paper = next(d for d in docs if "Tan et al" in d.path.name)
    cs = loaded.chunking.sliding_window

    chunks = chunk_sliding_window(
        paper.text,
        doc_id=paper.id,
        chunk_size=cs.chunk_size,
        overlap=cs.overlap,
        metadata={
            "titulo": paper.path.stem,
            "file_type": paper.metadata.get("file_type"),
            "subject": paper.metadata.get("subject"),
        },
    )

    mo.vstack(
        [
            mo.md(
                f"**{paper.path.name}** — {len(paper.text)} chars, "
                f"**{len(chunks)} chunks** `(chunk_size={cs.chunk_size}, overlap={cs.overlap})`"
            ),
            mo.md(f"**primer chunk:** `{chunks[0].text[:100]}…`"),
            mo.ui.table(
                [
                    {
                        "#": i,
                        "tokens": c.end_idx - c.start_idx,
                        "start": c.start_idx,
                        "end": c.end_idx,
                        "overlap_con_prev": c.start_idx - (chunks[i - 1].start_idx if i else 0),
                    }
                    for i, c in enumerate(chunks[:8])
                ],
                label="Primeros chunks (índices sobre tokens)",
            ),
            mo.md(f"**metadata propagada:** `{chunks[0].metadata}`"),
        ]
    )
    return (cs,)


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### Resumen — pipeline completo sobre la librería
    """)
    return


@app.cell
def _(chunk_sliding_window, cs, docs: "list[Document]", mo):
    per_doc = []
    for doc in docs:
        n = len(
            chunk_sliding_window(
                doc.text, doc_id=doc.id, chunk_size=cs.chunk_size, overlap=cs.overlap
            )
        )
        per_doc.append(
            {
                "archivo": doc.path.name[:55],
                "tipo": doc.metadata.get("file_type"),
                "chars": len(doc.text),
                "chunks": n,
            }
        )

    total_chunks = sum(r["chunks"] for r in per_doc)

    mo.vstack(
        [
            mo.md(
                f"**{len(docs)} documentos → {total_chunks} chunks** con "
                f"`chunk_size={cs.chunk_size}`, `overlap={cs.overlap}`"
            ),
            mo.ui.table(per_doc),
        ]
    )
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md("""
    ### (opcional) Scale check — PDF grande vía `make seed-large`
    """)
    return


@app.cell
def _(mo):
    run_scale = mo.ui.switch(
        value=False,
        label="Ejecutar scale check con `/tmp/lfx_rag_large.pdf` (generado con `make seed-large`)",
    )
    run_scale
    return (run_scale,)


@app.cell
def _(Path, chunk_sliding_window, mo, run_scale):
    def _eda_scale():
        if not run_scale.value:
            return mo.md("Switch apagado — no se ejecuta nada.")

        large = Path("/tmp/lfx_rag_large.pdf")
        if not large.exists():
            return mo.md("⚠️ `/tmp/lfx_rag_large.pdf` no existe — corre `make seed-large`.")

        import time

        import pymupdf

        start_t = time.perf_counter()
        with pymupdf.open(str(large)) as doc:
            text = "".join(page.get_text() for page in doc)
        extract_s = time.perf_counter() - start_t

        start_c = time.perf_counter()
        chunks = chunk_sliding_window(text, doc_id="scale", chunk_size=512, overlap=128)
        chunk_s = time.perf_counter() - start_c

        return mo.md(
            f"**{large.stat().st_size / 1e6:.1f} MB** → {len(text)} chars\n\n"
            f"- extracción de texto: `{extract_s:.2f}s`  \n"
            f"- chunking (512/128): `{chunk_s:.2f}s` → **{len(chunks)} chunks**"
        )

    _eda_scale()
    return


if __name__ == "__main__":
    app.run()
