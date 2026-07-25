-- lfx manifest schema v1
-- See docs/schema.md for full reference.

-- Schema version tracking
CREATE TABLE IF NOT EXISTS _migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Documents: one row per physical file, content-addressed by SHA-256
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,                    -- SHA-256(content), 64 hex chars
    source_path TEXT NOT NULL,              -- absolute path to original file
    file_type TEXT NOT NULL
        CHECK(file_type IN ('pdf','epub','djvu','docx','doc')),
    file_size INTEGER,                      -- bytes
    page_count INTEGER,                     -- nullable for DjVu/DOCX
    has_text_layer BOOLEAN DEFAULT FALSE,   -- TRUE = selectable text, no OCR needed
    needs_ocr BOOLEAN DEFAULT FALSE,        -- TRUE = needs OCR (Phase 3)
    quality_flags TEXT,                     -- JSON: ["blur","skew","low_contrast"]
    language TEXT,                          -- ISO code: "en","es","de","ja"
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP  -- auto-updated via trigger
);

CREATE INDEX IF NOT EXISTS idx_documents_file_type ON documents(file_type);
CREATE INDEX IF NOT EXISTS idx_documents_language ON documents(language);

-- Known types: valid types for UI and fuzzy matching
CREATE TABLE IF NOT EXISTS known_types (
    key TEXT PRIMARY KEY,
    label_en TEXT NOT NULL,
    label_es TEXT NOT NULL,
    is_default BOOLEAN DEFAULT TRUE
);

INSERT OR IGNORE INTO known_types (key, label_en, label_es) VALUES
    ('libro',    'Book',              'Libro'),
    ('paper',    'Paper / Article',   'Paper / Articulo'),
    ('tesis',    'Thesis',            'Tesis'),
    ('notas',    'Notes',             'Notas / Apuntes'),
    ('manual',   'Manual / Guide',    'Manual / Guia'),
    ('reporte',  'Report / Tech Doc', 'Reporte / Doc. tecnico'),
    ('personal', 'Personal Document', 'Documento personal'),
    ('otro',     'Other',             'Otro');

-- Subject groups: clusters of emergent subjects
CREATE TABLE IF NOT EXISTS subject_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    name_en TEXT,
    name_es TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Subject aliases: map LLM strings to groups
CREATE TABLE IF NOT EXISTS subject_aliases (
    alias TEXT PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES subject_groups(id) ON DELETE CASCADE,
    source TEXT DEFAULT 'auto'              -- "auto" (LLM) | "manual" (user)
        CHECK(source IN ('auto','manual')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO subject_groups (name, name_en, name_es) VALUES
    ('Physics',          'Physics',          'Fisica'),
    ('Computer Science', 'Computer Science',  'Ciencias de la Computacion'),
    ('Mathematics',      'Mathematics',       'Matematicas'),
    ('Engineering',      'Engineering',       'Ingenieria'),
    ('Medicine',         'Medicine',          'Medicina'),
    ('Business',         'Business',          'Negocios'),
    ('Other',            'Other',             'Otro');

-- Classifications: history of classification attempts
-- One accepted per document at a time (unique partial index)
CREATE TABLE IF NOT EXISTS classifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    source TEXT NOT NULL
        CHECK(source IN ('heuristic','llm','manual')),
    model TEXT,                             -- e.g. "qwen3:8b", "gpt-4", "user"
    tipo TEXT,                              -- type after fuzzy matching
    tipo_original TEXT,                     -- raw LLM output before matching
    subject TEXT,                           -- emergent, free field
    subject_hierarchy TEXT,                 -- JSON: ["Physics","Quantum Mechanics"]
    autor TEXT,
    titulo TEXT,
    anio TEXT,                              -- TEXT for "2020", "circa 1990", "s.f."
    editorial TEXT,
    edicion TEXT,
    description TEXT,
    confidence REAL DEFAULT 0.0,            -- 0.0-1.0
    accepted BOOLEAN DEFAULT FALSE,
    accepted_at TIMESTAMP,                  -- first approval (not updated)
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_classifications_document ON classifications(document_id);
CREATE INDEX IF NOT EXISTS idx_classifications_accepted ON classifications(accepted);
CREATE INDEX IF NOT EXISTS idx_classifications_tipo ON classifications(tipo);
CREATE INDEX IF NOT EXISTS idx_classifications_subject ON classifications(subject);
CREATE INDEX IF NOT EXISTS idx_classifications_titulo ON classifications(titulo);
CREATE INDEX IF NOT EXISTS idx_classifications_autor ON classifications(autor);

CREATE UNIQUE INDEX IF NOT EXISTS idx_one_accepted_per_doc
    ON classifications(document_id) WHERE accepted = TRUE;

-- Full-text search (FTS5, external content)
CREATE VIRTUAL TABLE IF NOT EXISTS classifications_fts USING fts5(
    titulo,
    autor,
    description,
    subject,
    content='classifications',
    content_rowid='id'
);

-- FTS sync triggers
CREATE TRIGGER IF NOT EXISTS classifications_ai AFTER INSERT ON classifications BEGIN
    INSERT INTO classifications_fts(rowid, titulo, autor, description, subject)
    VALUES (new.id, new.titulo, new.autor, new.description, new.subject);
END;

CREATE TRIGGER IF NOT EXISTS classifications_ad AFTER DELETE ON classifications BEGIN
    INSERT INTO classifications_fts(classifications_fts, rowid, titulo, autor, description, subject)
    VALUES('delete', old.id, old.titulo, old.autor, old.description, old.subject);
END;

CREATE TRIGGER IF NOT EXISTS classifications_au AFTER UPDATE ON classifications BEGIN
    INSERT INTO classifications_fts(classifications_fts, rowid, titulo, autor, description, subject)
    VALUES('delete', old.id, old.titulo, old.autor, old.description, old.subject);
    INSERT INTO classifications_fts(rowid, titulo, autor, description, subject)
    VALUES (new.id, new.titulo, new.autor, new.description, new.subject);
END;

-- Tags: multi-language tag system (status/priority/custom)
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    label_en TEXT,
    label_es TEXT,
    category TEXT NOT NULL DEFAULT 'custom'
        CHECK(category IN ('status','priority','custom'))
);

CREATE TABLE IF NOT EXISTS document_tags (
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (document_id, tag_id)
);

INSERT OR IGNORE INTO tags (key, label_en, label_es, category) VALUES
    ('to_read',    'To Read',      'Por leer',      'status'),
    ('reading',    'Reading',      'Leyendo',       'status'),
    ('finished',   'Finished',     'Terminado',     'status'),
    ('reference',  'Reference',    'Referencia',    'status'),
    ('urgent',     'Urgent',       'Urgente',       'priority'),
    ('optional',   'Optional',     'Opcional',      'priority'),
    ('favorite',   'Favorite',     'Favorito',      'custom'),
    ('loaned',     'Loaned',       'Prestado',      'custom'),
    ('outdated',   'Outdated',     'Desactualizado','custom');

-- Organization: result of lfx apply
CREATE TABLE IF NOT EXISTS organization (
    document_id TEXT PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    source_path TEXT NOT NULL,              -- original path before apply
    target_path TEXT NOT NULL,              -- final path in ~/Biblioteca/
    backup_path TEXT,                       -- backup before apply (for undo)
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Duplicates: links between duplicate documents
CREATE TABLE IF NOT EXISTS duplicates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    duplicate_of TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    similarity REAL NOT NULL,               -- 0.0-1.0 (1.0 = identical)
    reason TEXT
        CHECK(reason IN ('same_content','same_title','different_edition')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(document_id, duplicate_of)
);

CREATE INDEX IF NOT EXISTS idx_duplicates_document ON duplicates(document_id);

-- Trigger: auto-update updated_at
CREATE TRIGGER IF NOT EXISTS trg_documents_updated_at
    AFTER UPDATE ON documents
    FOR EACH ROW
BEGIN
    UPDATE documents SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Record initial migration
INSERT OR IGNORE INTO _migrations (version) VALUES (1);
