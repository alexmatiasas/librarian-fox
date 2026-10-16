# AGENTS.md — librarian_fox (lfx)

Repo: Go CLI that scans reading documents (PDF/DjVu/DOCX/DOC/EPUB) into a SQLite manifest. Only `scan`, `config`, `version` are implemented; `classify/restore/apply/serve` are planned.

## Build & test (exact commands)

Requires CGO (sqlite3 + FTS5). Use `just` (replaced Makefile).

Root (lfx — Go):
```bash
just build   # schema + CGO_ENABLED=1 go build -tags sqlite_fts5
just test    # CGO_ENABLED=1 go test -tags sqlite_fts5 -race -count=1 ./...
just lint    # golangci-lint run
just format  # gofumpt -w -extra .
just schema  # go generate ./internal/config/ (configs/lfx.schema.json)
just clean
```

`lfx-rag/` (Python, uv + ruff + pytest + pyright):
```bash
cd lfx-rag && just install  # uv sync --all-extras --dev
cd lfx-rag && just lint     # uv run ruff check .
cd lfx-rag && just format   # uv run ruff format .
cd lfx-rag && just typecheck # uv run pyright src
cd lfx-rag && just test     # uv run pytest -q
cd lfx-rag && just cov      # uv run pytest --cov=src --cov-report=term-missing -q
cd lfx-rag && just serve-api # uv run uvicorn lfx_rag.api:app --reload
```

Manual build:
```bash
CGO_ENABLED=1 go build -tags "sqlite_fts5" -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" -o lfx ./cmd/lfx
```

## CLI (implemented)

- `lfx version` — prints version/commit
- `lfx config` — print config, validate, show schema path, write example, show precedence
- `lfx scan [root]` — discover documents, populate SQLite manifest. Global flags: `--config`, `--db`, `--log-level`, `--json`, `--dry-run`. Scan flags: `--root`, `--exclude-dir`, `--no-default-excludes`, `--max-depth`, `--stats-only`, `--follow-symlinks`, `--min-size`, `--max-size`, `--include-ext`, `--exclude-ext`, `--lang`.

Note: other subcommands exist as stubs (commented) in `cmd/lfx/main.go`.

## Config

Precedence (highest → lowest): `--flags` > `$ENV_VARS` (see code) > `./lfx.toml` > `~/.config/lfx/lfx.toml` > defaults. Loaded via `internal/config` (TOML). Schema generated from `Config` struct (`cmd/genschema`) to `configs/lfx.schema.json` by `go generate ./internal/config/`.

## Data model / manifest

SQLite manifest at path from config (`[general].manifest_path`, defaults to `~/.local/state/lfx/manifest.db` in practice). Schema: `internal/manifest/migrations/001_initial.sql` defines tables (documents, classifications, organization, duplicates, tags, subject groups/aliases, FTS5). `internal/manifest` opens DB with `PRAGMA foreign_keys = ON`. Build requires `sqlite_fts5` tag. Content-addressed by SHA-256 (`documents.id`).

## Architecture notes (high-signal)

- CLI in `cmd/lfx/` (Cobra). Business logic in `internal/` — no `fmt.Print*`, no `os.Exit` there (interface-agnostic).
- External tools used via `os/exec`: Calibre (`ebook-meta`, `fetch-ebook-metadata`, `ebook-convert`, `calibredb`), `ocrmypdf`, `pdftotext`, `ghostscript`, `tesseract`, `djvutxt`/`ddjvu` (see `internal/metadata`, `internal/pdf`, `internal/djvu`, `internal/scan`).
- Scan pipeline: `internal/scan` walks tree, filters by extensions/size/excludes/depth, skips symlinks by default, computes SHA-256, extracts text/pages where possible, detects language, upserts to manifest. Dry-run supported.
- Concurrency: worker pools per phase with configurable limits (LLM/OCR). Context cancellation respected.
- Two components separated: lfx (Go root) and lfx-rag (Python subdir), each with independent justfile and tooling (uv for Python).

## Development gotchas

- CGO required. On macOS need `sqlite3` dev headers (Homebrew usually fine). Build must include `-tags "sqlite_fts5"`.
- `just build` depends on `schema` (go generate). If you change `internal/config` structs, regenerate schema.
- Tests use `-race -count=1`. Integration tests may touch filesystem/manifest paths; keep temp dirs isolated.
- Foreign keys: SQLite default off — code sets `PRAGMA foreign_keys = ON` on open.
- Symlinks: skipped unless `--follow-symlinks` (safety).
- Two justfiles: root (Go), `lfx-rag/` (Python/uv). Components independent.
- `just check` runs fast guard (root: lint + build + test; lfx-rag: ruff check + ruff format --check + pyright + pytest). Use after small changes.
- Task workflow: Plan→Build(in worktree, per step `just check`+commit)→Review(read-only, free model)→Verify→Merge to develop. Task files in `docs/tasks/NNN-*.md`.

## Quick start

```bash
# build (Go)
just build

# scan dry-run
./lfx scan ~/Documents --dry-run --json

# scan and persist to manifest
./lfx scan ~/

# Python (lfx-rag)
cd lfx-rag && just install && just test
```
