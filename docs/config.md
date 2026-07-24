# Configuration Reference

lfx uses a TOML configuration file. Priority order:

```text
hardcoded defaults < ~/.config/lfx/lfx.toml < ./lfx.toml < --flags < $ENV_VARS
```

**Config file locations**:

- Global: `~/.config/lfx/lfx.toml`
- Local (project): `./lfx.toml`

**Source**: `configs/lfx.toml`

**JSON Schema**: `configs/lfx.schema.json` (auto-generated from Go structs via `go generate`)

**VS Code integration**: Install the [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml) extension for autocompletion and validation.

---

## Config format

**Use section format**, not inline tables:

```toml
# ✅ Correct - section format
[apply]
filename_template = "{autor} - {titulo} ({anio})"
compression = "ocrmypdf"

# ❌ Wrong - inline table format (autocomplete may suggest this)
apply = { filename_template = "{autor} - {titulo} ({anio})", compression = "ocrmypdf" }
```

**All sections are optional** — only include the sections you want to override. Defaults are applied automatically.

**Template strings with curly braces** — work correctly inside quoted strings:

```toml
# ✅ Correct - curly braces inside quoted strings
filename_template = "{autor} - {titulo} ({anio})"
backup_dir = "{library_dir}/.backups"
```

**Note**: If autocomplete generates inline table format, delete it and use the section format `[section]` instead.

---

## [general]

| Option | Type | Default | Description |
|---|---|---|---|
| `root` | string | `~` | Root directory to scan. |
| `library_dir` | string | `~/Library` | Where lfx organizes the library. |
| `manifest_path` | string | `~/.local/share/lfx/manifest.db` | SQLite database location. |
| `log_level` | enum | `info` | `debug`, `info`, `warn`, or `error` |
| `dry_run` | bool | `true` | Safety mode. Only generates report of what would happen. |

---

## [scan]

Phase 1 - Discovery.

| Option | Type | Default | Description |
|---|---|---|---|
| `max_depth` | int | `5` | Max recursion depth from root. 5 covers `~/Documents/subfolder/file.pdf`. |
| `min_file_size_kb` | int | `10` | Ignore files smaller than this (thumbnails, junk). |
| `max_file_size_mb` | int | `500` | Ignore files larger than this (probably not books). |
| `exclude_dirs` | list | (see below) | Directories to skip (relative to root). Symlinks are always skipped. |
| `extensions` | list | `["pdf","djvu","docx","doc"]` | File extensions to process. `.md` and `.txt` are explicitly ignored. |

### Default excluded directories

```toml
exclude_dirs = [
    "Library", "node_modules", ".cache", ".Trash", ".npm", ".nvm",
    ".cargo", ".rustup", "go/pkg", "go/src", ".local/share/Trash",
    "Applications", "Movies", "Music", "Pictures", "Public", ".git", "Desktop",
]
```

---

## [llm]

Phase 2 - Classification.

| Option | Type | Default | Description |
|---|---|---|---|
| `provider` | enum | `ollama` | `ollama` (local), `groq` (cloud free), or `cerebras` (cloud free). |
| `model` | string | `qwen3:8b` | Model name. For Ollama: local model name. For cloud: provider model ID. |
| `fallback_model` | string | `llama3.3:8b` | Fallback model if primary fails or has low confidence. |
| `confidence_threshold` | float | `0.7` | Below this -> `unknown` status. 0.0 = accept all, 1.0 = only perfect confidence. |
| `confidence_warning` | float | `0.9` | Between threshold and warning -> "low confidence" warning. |
| `temperature` | float | `0.2` | LLM temperature. Lower = more deterministic (recommended for classification). |
| `max_concurrent` | int | `3` | Parallel LLM calls. Depends on provider: Ollama 1-3, Groq 10-20. |
| `timeout_seconds` | int | `60` | Timeout per LLM call in seconds. |
| `local_only_categories` | list | `["Personal / Administrativo"]` | Documents in these categories always processed with local Ollama. |
| `retry` | bool | `true` | Enable retry with fallback model on low confidence. |
| `max_retries` | int | `2` | Maximum retry attempts (same model + fallback). |

### Confidence workflow

```
Document -> LLM -> confidence: 0.65
                          |
                          v
                confidence >= 0.7? --NO--> retry?
                          |                  |
                         YES                NO --> manual review
                          |                  |
                          v                 YES
                confidence >= 0.9?          |
                          |                  v
                         NO            retry same model
                          |                  |
                          v                  v
                    warning         confidence >= 0.7?
                                            |
                                           NO
                                            |
                                            v
                                   retry different model
                                            |
                                            v
                                   confidence >= 0.7?
                                            |
                                           NO
                                            |
                                            v
                                   manual review
```

### API key (environment variable only)

```bash
LFX_API_KEY=sk-...  # For cloud providers (Groq/Cerebras)
```

---

## [library]

Phase 2 - Taxonomy.

| Option | Type | Default | Description |
|---|---|---|---|
| `layout` | string | `{tipo}/{subject}` | Folder layout. Variables: `{tipo}`, `{subject}`, `{autor}`, `{titulo}`, `{anio}` |

### Layout examples

| Pattern | Result |
|---|---|
| `{tipo}/{subject}` | `Libros/Physics/Griffiths - Quantum Mechanics (2018).pdf` |
| `{subject}/{tipo}` | `Physics/Libros/Griffiths - Quantum Mechanics (2018).pdf` |
| `{tipo}` | `Libros/Griffiths - Quantum Mechanics (2018).pdf` |
| `flat` | `Griffiths - Quantum Mechanics (2018).pdf` (no subdirectories) |

### [library.types]

Fixed document types. LLM assigns one of these to each document.

```toml
valid_types = [
    "Libro", "Paper / Artículo", "Tesis", "Notas / Apuntes",
    "Manual / Guía", "Reporte / Doc. técnico", "Documento personal", "Otro",
]
```

Users can extend this list by adding custom types to their config.

### [library.subjects]

| Option | Type | Default | Description |
|---|---|---|---|
| `strategy` | enum | `emergente` | `emergente` (LLM creates freely), `predefinida` (whitelist), or `mixta` |
| `merge_threshold` | float | `0.8` | Similarity threshold to auto-group subjects. |

**Note**: In v1, only `emergente` strategy is implemented.

---

## [metadata]

Phase 2 - Calibre CLI integration.

| Option | Type | Default | Description |
|---|---|---|---|
| `ebook_meta_path` | string | `ebook-meta` | Path to Calibre's `ebook-meta` CLI. Auto-detected on macOS/Linux. |
| `fetch_metadata_path` | string | `fetch-ebook-metadata` | Path to Calibre's `fetch-ebook-metadata` CLI. |
| `metadata_sources` | list | `["google_books","open_library"]` | Sources to cross-reference LLM results. |

---

## [ocr]

Phase 3 - Scan restoration.

| Option | Type | Default | Description |
|---|---|---|---|
| `languages` | list | `["eng","spa"]` | Tesseract language packs. Document language is auto-detected; these are fallbacks. |
| `max_concurrent` | int | `2` | Parallel OCR jobs. CPU-intensive: don't exceed available CPU cores. |

### [ocr.quality]

Quality triage thresholds.

| Option | Type | Default | Description |
|---|---|---|---|
| `blur_threshold` | float | `100.0` | Laplacian variance. Lower = blurrier. Files below this need OCR preprocessing. |
| `skew_threshold` | float | `2.0` | Max acceptable skew in degrees. Above this -> deskew before OCR. |
| `contrast_min_spread` | float | `30.0` | Minimum histogram spread. Lower = low contrast. |

---

## [apply]

Phase 4 - Apply changes.

| Option | Type | Default | Description |
|---|---|---|---|
| `filename_template` | string | `{autor} - {titulo} ({anio})` | Output filename. Variables: `{autor}`, `{titulo}`, `{anio}`, `{subject}`, `{tipo}` |
| `compression` | enum | `ocrmypdf` | `ocrmypdf` (OCR + compress), `ghostscript` (compress only), or `none` |
| `conflict` | enum | `suffix` | How to handle filename conflicts: `suffix`, `overwrite`, `skip`, or `ask` |
| `backup_dir` | string | `{library_dir}/.backups` | Where to store backups before apply. |
| `backup_compress` | bool | `true` | Compress backup files (gzip). |
| `backup_max_count` | int | `5` | Maximum number of backups to keep. |
| `backup_max_age_days` | int | `30` | Delete backups older than this (days). |

---

## [kavita]

Phase 5 - Multi-device access.

| Option | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Enable Kavita server (disabled in v1). |
| `docker_compose_path` | string | `~/.config/lfx/docker-compose.kavita.yml` | Docker Compose file location. |
| `port` | int | `5000` | Kavita web UI port. |

---

## Future options (post-v1)

These options are defined but have no effect in v1.

```toml
# [tailscale]
# enabled = false   # Secure remote access to Kavita

# [sync]
# enabled = false   # Syncthing: sync subfolder to phone

# [apply]
# zotero_compat = false  # Soft links in ~/Library/.zotero-links/
```

---

## JSON Schema (Auto-generated)

The JSON schema is auto-generated from Go structs:

```bash
go generate ./internal/config/
```

This regenerates `configs/lfx.schema.json` from the struct definitions in `internal/config/config.go`.

**VS Code integration**: Install the [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml) extension for autocompletion and validation.

---

## Known issues

### VS Code autocomplete mangles curly braces

**Affected fields**: `layout`, `filename_template`, `backup_dir`

**Problem**: The Even Better TOML extension's autocomplete removes the closing `}` when inserting template variables and adds it to the end:

```toml
# Autocomplete inserts:
layout = "{tipo/{subject}"}

# Should be:
layout = "{tipo}/{subject}"
```

**Workaround**: Type the value manually instead of using autocomplete, or verify after autocomplete completes.

This is a limitation of the Even Better TOML extension, not a bug in lfx.
