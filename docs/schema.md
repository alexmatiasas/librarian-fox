# Schema Reference

SQLite manifest database for lfx. Single file, zero config, portable.

**Migration**: `internal/manifest/migrations/001_initial.sql`

## Tables

| Table | Purpose | Typical rows |
|---|---|---|
| `_migrations` | Schema version tracking | 1-5 |
| `documents` | Unique identity per file (content-addressed) | 166 |
| `known_types` | Valid types for UI and fuzzy matching | 8 |
| `subject_groups` | Groups for emergent subjects | 7-50 |
| `subject_aliases` | Maps LLM strings to groups | 0-200 |
| `classifications` | Classification attempts history | 166-830 |
| `classifications_fts` | Full-text search (FTS5, external content) | mirror |
| `tags` | Multi-language tags (status/priority/custom) | 9-50 |
| `document_tags` | Many-to-many document-tag | 0-830 |
| `organization` | Where each document ended up after apply | 0-166 |
| `duplicates` | Links between duplicate documents | 0-50 |

---

## documents

Unique identity of each file. One row per physical file. Immutable once created.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | TEXT | yes | — | SHA-256 of file content (64 hex chars). Computed by Go, not auto-generated. |
| `source_path` | TEXT | yes | — | Absolute path to original file. |
| `file_type` | TEXT | yes | — | `pdf`, `epub`, `djvu`, `docx`, `doc` |
| `file_size` | INTEGER | no | — | Size in bytes. |
| `page_count` | INTEGER | no | — | Number of pages. Nullable (DjVu/DOCX don't have "pages"). |
| `has_text_layer` | BOOLEAN | no | FALSE | TRUE = has selectable text, no OCR needed. |
| `needs_ocr` | BOOLEAN | no | FALSE | TRUE = needs OCR (Phase 3). |
| `quality_flags` | TEXT | no | — | JSON array: `["blur","skew","low_contrast"]` |
| `language` | TEXT | no | — | ISO code: `en`, `es`, `de`, `ja`. Auto-detected. |
| `created_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | Auto-generated. |
| `updated_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | Auto-updated via trigger. |

**Constraints**: PRIMARY KEY (`id`), CHECK (`file_type IN (...)`)
**Indexes**: `file_type`, `language`

### Design decisions

- **Content-addressed**: `id` = SHA-256(content). Same content = same ID. Detects duplicates automatically.
- **TEXT for paths**: More universal than INTEGER for cross-platform paths.
- **No `status` column**: State is derived from existence of rows in other tables.

---

## known_types

Valid document types for UI and fuzzy matching. LLM proposes freely, Go fuzzy-matches against this table.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `key` | TEXT | yes | — | Machine identifier: `libro`, `paper`, `tesis`, etc. |
| `label_en` | TEXT | yes | — | English display name. |
| `label_es` | TEXT | yes | — | Spanish display name. |
| `is_default` | BOOLEAN | no | TRUE | Show in UI by default. |

### Default types

| Key | English | Spanish |
|---|---|---|
| `libro` | Book | Libro |
| `paper` | Paper / Article | Paper / Articulo |
| `tesis` | Thesis | Tesis |
| `notas` | Notes | Notas / Apuntes |
| `manual` | Manual / Guide | Manual / Guia |
| `reporte` | Report / Tech Doc | Reporte / Doc. tecnico |
| `personal` | Personal Document | Documento personal |
| `otro` | Other | Otro |

---

## subject_groups

Groups for emergent subjects. After classification, lfx groups similar subjects. User can merge/rename groups.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | INTEGER | yes | AUTOINCREMENT | Primary key. |
| `name` | TEXT | yes | — | Group name (unique). |
| `name_en` | TEXT | no | — | English display name. |
| `name_es` | TEXT | no | — | Spanish display name. |
| `created_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | Auto-generated. |

### Default groups

Physics, Computer Science, Mathematics, Engineering, Medicine, Business, Other.

---

## subject_aliases

Maps LLM output strings to subject groups. E.g., "Quantum Mechanics" -> group "Physics".

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `alias` | TEXT | yes | — | Alias string (primary key). |
| `group_id` | INTEGER | yes | — | FK to `subject_groups`. CASCADE delete. |
| `source` | TEXT | no | `auto` | `auto` (LLM grouped) or `manual` (user). |
| `created_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | Auto-generated. |

**Constraints**: FK `group_id` -> `subject_groups(id)` ON DELETE CASCADE

---

## classifications

History of classification attempts. Each row = one attempt (LLM, heuristic, or manual). Only one `accepted` per document at a time.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | INTEGER | yes | AUTOINCREMENT | Primary key. |
| `document_id` | TEXT | yes | — | FK to `documents`. CASCADE delete. |
| `source` | TEXT | yes | — | `heuristic`, `llm`, or `manual` |
| `model` | TEXT | no | — | Model used: `qwen3:8b`, `gpt-4`, `user` |
| `tipo` | TEXT | no | — | Assigned type (after fuzzy matching). |
| `tipo_original` | TEXT | no | — | Raw LLM output before fuzzy matching. |
| `subject` | TEXT | no | — | Emergent subject (free field). |
| `subject_hierarchy` | TEXT | no | — | JSON: `["Physics","Quantum Mechanics"]` |
| `autor` | TEXT | no | — | Author name. |
| `titulo` | TEXT | no | — | Title. |
| `anio` | TEXT | no | — | Year. TEXT to support "2020", "circa 1990", "s.f." |
| `editorial` | TEXT | no | — | Publisher. |
| `edicion` | TEXT | no | — | Edition. |
| `description` | TEXT | no | — | Content summary. |
| `confidence` | REAL | no | 0.0 | 0.0-1.0. Source-dependent: heuristic=1.0, llm=score, manual=1.0 |
| `accepted` | BOOLEAN | no | FALSE | User approved this classification. |
| `accepted_at` | TIMESTAMP | no | — | First approval timestamp (not updated). |
| `reason` | TEXT | no | — | Why it was changed (if applicable). |
| `created_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | Auto-generated. |

**Constraints**: FK `document_id` -> `documents(id)` ON DELETE CASCADE, CHECK (`source IN (...)`), UNIQUE partial (`document_id WHERE accepted = TRUE`)
**Indexes**: `document_id`, `accepted`, `tipo`, `subject`, `titulo`, `autor`

### Design decisions

- **Separate table**: Not a column on `documents`. Allows history of multiple attempts, fuzzy matching, and approval tracking.
- **Hybrid type**: LLM proposes freely (`tipo_original`), Go fuzzy-matches against `known_types` -> `tipo`. No CHECK constraint for flexibility.
- **Emergent subjects**: Free field, no whitelist. `subject_groups` + `subject_aliases` group after classification.

---

## classifications_fts

Full-text search index for classifications. External content table: FTS5 only stores the index, data lives in `classifications`.

| Column | Type | Description |
|---|---|---|
| `titulo` | TEXT | Title (indexed). |
| `autor` | TEXT | Author (indexed). |
| `description` | TEXT | Description (indexed). |
| `subject` | TEXT | Subject (indexed). |

**Triggers**: `classifications_ai`, `classifications_ad`, `classifications_au` keep FTS in sync.

---

## tags

Multi-language tag system. Categories: `status`, `priority`, `custom`.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | INTEGER | yes | AUTOINCREMENT | Primary key. |
| `key` | TEXT | yes | — | Machine identifier (unique). |
| `label_en` | TEXT | no | — | English display name. |
| `label_es` | TEXT | no | — | Spanish display name. |
| `category` | TEXT | yes | `custom` | `status`, `priority`, or `custom` |

### Default tags

| Key | English | Spanish | Category |
|---|---|---|---|
| `to_read` | To Read | Por leer | status |
| `reading` | Reading | Leyendo | status |
| `finished` | Finished | Terminado | status |
| `reference` | Reference | Referencia | status |
| `urgent` | Urgent | Urgente | priority |
| `optional` | Optional | Opcional | priority |
| `favorite` | Favorite | Favorito | custom |
| `loaned` | Loaned | Prestado | custom |
| `outdated` | Outdated | Desactualizado | custom |

---

## document_tags

Many-to-many relationship between documents and tags.

| Column | Type | Required | Description |
|---|---|---|---|
| `document_id` | TEXT | yes | FK to `documents`. CASCADE delete. |
| `tag_id` | INTEGER | yes | FK to `tags`. CASCADE delete. |
| `created_at` | TIMESTAMP | no | Auto-generated. |

**Constraints**: PRIMARY KEY (`document_id`, `tag_id`), FKs with CASCADE delete.

---

## organization

Result of `lfx apply`. Tracks where each document ended up after organizing.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `document_id` | TEXT | yes | — | FK to `documents`. CASCADE delete. Primary key. |
| `source_path` | TEXT | yes | — | Original path before apply. |
| `target_path` | TEXT | yes | — | Final path in `~/Biblioteca/`. |
| `backup_path` | TEXT | no | — | Backup before apply (for undo). |
| `applied_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | When it was applied. |

### Design decisions

- **`source_path`**: Tracks original location. Detects if user moved files manually after apply.
- **`backup_path`**: Enables undo. Backup created before moving files.

---

## duplicates

Links between duplicate documents. A document can be duplicated from multiple others.

| Column | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | INTEGER | yes | AUTOINCREMENT | Primary key. |
| `document_id` | TEXT | yes | — | FK to `documents`. CASCADE delete. |
| `duplicate_of` | TEXT | yes | — | FK to `documents`. CASCADE delete. |
| `similarity` | REAL | yes | — | 0.0-1.0 (1.0 = identical content). |
| `reason` | TEXT | no | — | `same_content`, `same_title`, or `different_edition` |
| `created_at` | TIMESTAMP | no | CURRENT_TIMESTAMP | Auto-generated. |

**Constraints**: UNIQUE (`document_id`, `duplicate_of`), CHECK (`reason IN (...)`)
**Indexes**: `document_id`

### Similarity values

- `1.0` = identical content (SHA-256 match)
- `< 1.0` = fuzzy match (same title, different edition)

---

## Triggers

| Trigger | Table | Event | Purpose |
|---|---|---|---|
| `trg_documents_updated_at` | `documents` | AFTER UPDATE | Auto-update `updated_at` timestamp |
| `classifications_ai` | `classifications` | AFTER INSERT | Sync new row to FTS index |
| `classifications_ad` | `classifications` | AFTER DELETE | Remove deleted row from FTS index |
| `classifications_au` | `classifications` | AFTER UPDATE | Update row in FTS index |

---

## Critical: PRAGMA foreign_keys = ON

SQLite has foreign keys **OFF by default**. Without enabling the pragma, `ON DELETE CASCADE` does not work and `REFERENCES` is not enforced.

```go
// In internal/manifest/db.go, when opening the connection:
db.Exec("PRAGMA foreign_keys = ON")
```

```bash
# In SQL testing:
sqlite3 /tmp/lfx_test.db "PRAGMA foreign_keys = ON;"
```

Without this, deleting a document does NOT delete its classifications, tags, organization, or duplicates. The schema is correct but SQLite does not enforce it without the pragma.
