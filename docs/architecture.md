# Architecture

lfx (librarian_fox) is a CLI tool that organizes a messy collection of reading documents into a unified, classified, restored, and accessible library.

## Problem

A typical user has hundreds of reading documents (PDFs, papers, theses, notes, multifunctional scans) scattered across their home directory. They don't know what they have, where each file is, and scans are low quality.

## Solution

```
~/ (166 files scattered, 528MB, 4 formats)
  -> lfx scan       (inventory: what do I have, where is it, what state is it in)
  -> lfx classify    (LLM classifies by type + emergent subject)
  -> lfx restore     (OCR of low-quality scans)
  -> lfx apply       (move/rename to ~/Biblioteca/ using configurable layout)
  -> Kavita          (serves the library via web + OPDS from any device)
```

## Design Principles

1. **Nothing moves without explicit human approval.** Every phase runs in dry-run mode first, generating a report of "what would happen." The user reviews before applying.
2. **Unknown files are never touched.** They are not moved, renamed, or processed.
3. **Resume-safe manifest.** If interrupted, it doesn't reprocess from scratch.
4. **DB as source of truth.** Folder structure is a derived view, configurable via `[library.layout]`. Reorganizing = edit DB + run `lfx apply`.

## Architecture Overview

```
lfx (Go CLI)
  |
  +-- internal/config       (TOML loading, defaults, validation)
  +-- internal/manifest      (SQLite: read, write, schema, migrations)
  +-- internal/llm           (provider interface + Ollama/cloud implementations)
  +-- internal/metadata      (Calibre CLI wrappers: ebook-meta, fetch-ebook-metadata)
  +-- internal/pdf           (wrappers: pdftotext, ocrmypdf, ghostscript)
  +-- internal/djvu          (wrappers: djvulibre)
  +-- internal/epub          (metadata parsing)
  +-- internal/quality       (scan quality triage)
  +-- internal/dedup         (content-hash deduplication: SHA-256)
```

## Technology Choices

| Component | Language | Why |
|---|---|---|
| **lfx** (this repo) | Go | Process orchestration, concurrency, CLI, single binary |
| **lfx-rag** (separate repo) | Python | ML ecosystem (embeddings, vector store, retrieval) |

### Go demonstrates

- Systems engineering, goroutines, channels for LLM calls and subprocesses
- CLI with Cobra, SQLite, subprocess orchestration, resume-safe pipeline
- Interface-agnostic `internal/` (no `fmt.Print*`, no `os.Exit`, returns structs)

### External tools (via `os/exec`)

- **Calibre** (`ebook-meta`, `fetch-ebook-metadata`, `ebook-convert`, `calibredb`): metadata, conversion, database
- **ocrmypdf**: OCR + deskew + compression
- **pdftotext**: text extraction
- **ghostscript**: PDF compression
- **tesseract**: OCR engine
- **djvulibre**: DjVu support

### Why Calibre as backend

Calibre's metadata plugins have been battle-tested since 2006. Reimplementing search against Google Books/Open Library/Amazon in Go would be repetitive work without value. Calibre already handles format conversion, metadata read/write, and SQLite database. lfx adds what Calibre doesn't: OCR triage, LLM classification, content-based dedup, resume-safe pipeline, and Kavita+OPDS layer.

## Pipeline Phases

### Phase 0 - Validation

Before building the rest: run `ocrmypdf` against 2-3 real multifunctional scan PDFs to confirm the approach works before investing in the full pipeline.

### Phase 1 - Discovery & Inventory

- Traverse `~/` (`filepath.WalkDir`), filter by configured extensions
- Exclude configured directories, skip symlinks
- Apply size filters
- For each file: record path, size, SHA-256, page count, text layer presence
- Generate initial manifest in SQLite

### Phase 2 - Classification & Metadata

- Heuristic filter first: ISBN/DOI in metadata discard obvious cases without LLM
- For remaining documents: LLM call with first page + metadata, requesting structured JSON
- Fuzzy matching against `known_types` for consistent type assignment
- Emergent subjects (no forced whitelist)
- Cross-reference with Calibre metadata sources
- Interactive mode: LLM proposes, user approves/rejects live

### Phase 3 - Scan Restoration

- For documents without text layer: quality triage per page
- Triage: blur (Laplacian variance), skew (Hough transform), contrast (histogram spread)
- Execute `ocrmypdf` with auto-detected languages
- Parallel OCR with configurable concurrency

### Phase 4 - Apply Changes

- Automatic backup before moving files
- User reviews and corrects dry-run report
- Move/rename to `~/Biblioteca/` using configurable layout
- Compress according to configuration
- Record in manifest with status `applied`

### Phase 5 - Multi-device Access

```
~/Biblioteca/<Type>/<Subject>/Autor - Titulo (Ano).pdf
        |
    Kavita (Docker)  -- scans ~/Biblioteca/ automatically
        |
    OPDS reader (justRead, Panels, Moon+)  -- offline reading
```

## Configuration

TOML file at `~/.config/lfx/lfx.toml` or `./lfx.toml`. See [config.md](config.md) for all options.

## Database

SQLite manifest with 11 tables. See [schema.md](schema.md) for full reference.

### Key design decisions

- **Content-addressed IDs**: `id` = SHA-256(content). Same content = same ID. Auto-detects duplicates.
- **Classification as separate table**: Allows history of multiple attempts, fuzzy matching, approval tracking.
- **Emergent subjects**: LLM discovers subjects freely. `subject_groups` + `subject_aliases` group after classification.
- **DB as source of truth**: Folder structure is derived from DB. Reorganizing = edit DB + `lfx apply`.

### Critical: PRAGMA foreign_keys = ON

SQLite has foreign keys OFF by default. Without this pragma, CASCADE delete does not work. See [schema.md](schema.md#critical-pragma-foreign_keys--on) for details.

## Security

1. **Dry-run by default.** Always generate report before applying changes.
2. **Unknown files never touched.** Not moved, renamed, or processed.
3. **Resume-safe manifest.** No reprocessing from scratch.
4. **Never commit API keys.** Use environment variables.
5. **Skip symlinks.** `filepath.WalkDir` doesn't follow symlinks.
6. **Backup before apply.** `lfx apply` creates backup before moving files.

## Platform

- **v1**: macOS (Homebrew, Docker, Ollama)
- **v1.1**: Linux (same dependencies, different package manager)
- **Future**: Windows (verify Calibre CLI + Docker Desktop compatibility)

## Dependencies (macOS, via Homebrew)

```bash
# Core
brew install calibre docker ollama

# OCR (Phase 3)
brew install ocrmypdf tesseract tesseract-lang

# PDF tools (Phase 1 & 3)
brew install poppler ghostscript

# DjVu support
brew install djvulibre
```

### Ollama (local, for Phase 2)

```bash
ollama pull qwen3:8b   # Recommended for 16GB RAM
```
