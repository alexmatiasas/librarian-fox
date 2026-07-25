// Package manifest handles the SQLite database for document metadata.
//
// The manifest is the source of truth for all document information.
// Folder structure in ~/Biblioteca/ is a derived view of this database.
package manifest

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps sql.DB with lfx-specific methods.
type DB struct {
	*sql.DB
}

// Open creates or opens a SQLite database at the given path.
// It enables WAL mode and foreign keys.
func Open(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := mkdirAll(dir); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	dsn := path + "?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Set connection pool settings for SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	return &DB{db}, nil
}

// Migrate runs pending migrations from the embedded migrations directory.
func (db *DB) Migrate(ctx context.Context) error {
	// Ensure _migrations table exists
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM _migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("reading migration version: %w", err)
	}

	// List migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	// Sort by version number
	sort.Slice(entries, func(i, j int) bool {
		vi := extractVersion(entries[i].Name())
		vj := extractVersion(entries[j].Name())
		return vi < vj
	})

	// Apply pending migrations
	for _, entry := range entries {
		version := extractVersion(entry.Name())
		if version <= currentVersion {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}

		// Execute migration in a transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning migration transaction: %w", err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO _migrations (version) VALUES (?)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// InsertDocument inserts or updates a document in the database.
func (db *DB) InsertDocument(ctx context.Context, doc *Document) error {
	_, err := db.ExecContext(
		ctx, `
		INSERT INTO documents (
			id, source_path, file_type, file_size, page_count,
			has_text_layer, needs_ocr, quality_flags, language
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_path = excluded.source_path,
			file_type = excluded.file_type,
			file_size = excluded.file_size,
			page_count = excluded.page_count,
			has_text_layer = excluded.has_text_layer,
			needs_ocr = excluded.needs_ocr,
			quality_flags = excluded.quality_flags,
			language = excluded.language`,
		doc.ID, doc.SourcePath, doc.FileType, doc.FileSize, doc.PageCount,
		doc.HasTextLayer, doc.NeedsOCR, doc.QualityFlags, doc.Language,
	)
	if err != nil {
		return fmt.Errorf("inserting document: %w", err)
	}
	return nil
}

// scanDocNullable scans nullable database fields into Document pointer fields.
func scanDocNullable(
	pageCount sql.NullInt64,
	qualityFlags, language sql.NullString,
) (*int, *string, *Language) {
	var pCount *int
	var qFlags *string
	var lang *Language

	if pageCount.Valid {
		v := int(pageCount.Int64)
		pCount = &v
	}
	if qualityFlags.Valid {
		qFlags = &qualityFlags.String
	}
	if language.Valid {
		l := Language(language.String)
		lang = &l
	}

	return pCount, qFlags, lang
}

// GetDocument retrieves a document by its ID (SHA-256 hash).
func (db *DB) GetDocument(ctx context.Context, id string) (*Document, error) {
	doc := &Document{}
	var pageCount sql.NullInt64
	var qualityFlags, language sql.NullString

	err := db.QueryRowContext(ctx, `
		SELECT id, source_path, file_type, file_size, page_count,
		       has_text_layer, needs_ocr, quality_flags, language,
		       created_at, updated_at
		FROM documents WHERE id = ?`, id).Scan(
		&doc.ID, &doc.SourcePath, &doc.FileType, &doc.FileSize, &pageCount,
		&doc.HasTextLayer, &doc.NeedsOCR, &qualityFlags, &language,
		&doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, fmt.Errorf("getting document: %w", err)
	}

	doc.PageCount, doc.QualityFlags, doc.Language = scanDocNullable(pageCount, qualityFlags, language)

	return doc, nil
}

// ListDocuments returns documents in the database with optional pagination.
// Use limit=0, offset=0 to return all documents.
func (db *DB) ListDocuments(ctx context.Context, limit, offset int) ([]*Document, error) {
	query := `
		SELECT id, source_path, file_type, file_size, page_count,
		       has_text_layer, needs_ocr, quality_flags, language,
		       created_at, updated_at
		FROM documents ORDER BY source_path`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
		if offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", offset)
		}
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var docs []*Document
	for rows.Next() {
		doc := &Document{}
		var pageCount sql.NullInt64
		var qualityFlags, language sql.NullString

		if err := rows.Scan(
			&doc.ID, &doc.SourcePath, &doc.FileType, &doc.FileSize, &pageCount,
			&doc.HasTextLayer, &doc.NeedsOCR, &qualityFlags, &language,
			&doc.CreatedAt, &doc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}

		doc.PageCount, doc.QualityFlags, doc.Language = scanDocNullable(pageCount, qualityFlags, language)

		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

// InsertClassification inserts a new classification attempt.
func (db *DB) InsertClassification(ctx context.Context, c *Classification) error {
	result, err := db.ExecContext(
		ctx, `
		INSERT INTO classifications (
			document_id, source, model, tipo, tipo_original,
			subject, subject_hierarchy, autor, titulo, anio,
			editorial, edicion, description, confidence, accepted, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.DocumentID, c.Source, c.Model, c.Tipo, c.TipoOriginal,
		c.Subject, c.SubjectHierarchy, c.Autor, c.Titulo, c.Anio,
		c.Editorial, c.Edicion, c.Description, c.Confidence, c.Accepted, c.Reason,
	)
	if err != nil {
		return fmt.Errorf("inserting classification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting classification ID: %w", err)
	}
	c.ID = id

	return nil
}

// GetAcceptedClassification returns the accepted classification for a document, if any.
func (db *DB) GetAcceptedClassification(ctx context.Context, documentID string) (*Classification, error) {
	c := &Classification{}
	var source string
	var model, tipo, tipoOriginal, subject, subjectHierarchy sql.NullString
	var autor, titulo, anio, editorial, edicion, description, reason sql.NullString
	var acceptedAt sql.NullTime

	err := db.QueryRowContext(ctx, `
		SELECT id, document_id, source, model, tipo, tipo_original,
		       subject, subject_hierarchy, autor, titulo, anio,
		       editorial, edicion, description, confidence, accepted,
		       accepted_at, reason, created_at
		FROM classifications
		WHERE document_id = ? AND accepted = TRUE
		LIMIT 1`, documentID).Scan(
		&c.ID, &c.DocumentID, &source, &model, &tipo, &tipoOriginal,
		&subject, &subjectHierarchy, &autor, &titulo, &anio,
		&editorial, &edicion, &description, &c.Confidence, &c.Accepted,
		&acceptedAt, &reason, &c.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no accepted classification for document: %s", documentID)
		}
		return nil, fmt.Errorf("getting accepted classification: %w", err)
	}

	c.Source = ClassSource(source)
	cn := scanClassificationNullable(model, tipo, tipoOriginal, subject, subjectHierarchy,
		autor, titulo, anio, editorial, edicion, description, reason, acceptedAt)
	c.Model = cn.Model
	c.Tipo = cn.Tipo
	c.TipoOriginal = cn.TipoOriginal
	c.Subject = cn.Subject
	c.SubjectHierarchy = cn.SubjectHierarchy
	c.Autor = cn.Autor
	c.Titulo = cn.Titulo
	c.Anio = cn.Anio
	c.Editorial = cn.Editorial
	c.Edicion = cn.Edicion
	c.Description = cn.Description
	c.Reason = cn.Reason
	c.AcceptedAt = cn.AcceptedAt

	return c, nil
}

// AcceptClassification marks a classification as accepted and unaccepts others for the same document.
func (db *DB) AcceptClassification(ctx context.Context, id int64) error {
	// Get the document ID for this classification
	var documentID string
	err := db.QueryRowContext(ctx, "SELECT document_id FROM classifications WHERE id = ?", id).Scan(&documentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("classification not found: %d", id)
		}
		return fmt.Errorf("getting classification document: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Unaccept all other classifications for this document
	if _, err := tx.ExecContext(ctx, "UPDATE classifications SET accepted = FALSE WHERE document_id = ? AND accepted = TRUE", documentID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("unaccepting other classifications: %w", err)
	}

	// Accept the specified classification
	if _, err := tx.ExecContext(ctx, "UPDATE classifications SET accepted = TRUE, accepted_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("accepting classification: %w", err)
	}

	return tx.Commit()
}

// UpsertSubjectGroup creates or gets a subject group by name.
func (db *DB) UpsertSubjectGroup(ctx context.Context, name string) (*SubjectGroup, error) {
	// Try to get existing
	group := &SubjectGroup{}
	var nameEN, nameES sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT id, name, name_en, name_es, created_at
		FROM subject_groups WHERE name = ?`, name).Scan(
		&group.ID, &group.Name, &nameEN, &nameES, &group.CreatedAt,
	)
	if err == nil {
		if nameEN.Valid {
			group.NameEN = &nameEN.String
		}
		if nameES.Valid {
			group.NameES = &nameES.String
		}
		return group, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting subject group: %w", err)
	}

	// Create new
	result, err := db.ExecContext(ctx, "INSERT INTO subject_groups (name) VALUES (?)", name)
	if err != nil {
		return nil, fmt.Errorf("inserting subject group: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting subject group ID: %w", err)
	}

	group.ID = id
	group.Name = name
	return group, nil
}

// AddSubjectAlias maps an alias to a subject group.
func (db *DB) AddSubjectAlias(ctx context.Context, alias string, groupID int64, source AliasSource) error {
	_, err := db.ExecContext(ctx, `
		INSERT OR REPLACE INTO subject_aliases (alias, group_id, source)
		VALUES (?, ?, ?)`, alias, groupID, string(source))
	if err != nil {
		return fmt.Errorf("adding subject alias: %w", err)
	}
	return nil
}

// RecordOrganization records where a document was moved/renamed.
func (db *DB) RecordOrganization(ctx context.Context, o *Organization) error {
	_, err := db.ExecContext(ctx, `
		INSERT OR REPLACE INTO organization (document_id, source_path, target_path, backup_path)
		VALUES (?, ?, ?, ?)`, o.DocumentID, o.SourcePath, o.TargetPath, o.BackupPath)
	if err != nil {
		return fmt.Errorf("recording organization: %w", err)
	}
	return nil
}

// MarkDuplicate records that two documents are duplicates.
func (db *DB) MarkDuplicate(ctx context.Context, d *Duplicate) error {
	result, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO duplicates (document_id, duplicate_of, similarity, reason)
		VALUES (?, ?, ?, ?)`, d.DocumentID, d.DuplicateOf, d.Similarity, d.Reason)
	if err != nil {
		return fmt.Errorf("marking duplicate: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting duplicate status: %w", err)
	}
	if rowsAffected == 0 {
		return nil // already exists
	}

	id, _ := result.LastInsertId()
	d.ID = id

	return nil
}

// sanitizeFTSQuery wraps a user query in quotes to treat it as a literal phrase search.
// This prevents FTS5 operator injection (e.g., "* OR 1=1").
func sanitizeFTSQuery(query string) string {
	// Escape internal double quotes by doubling them
	escaped := strings.ReplaceAll(query, `"`, `""`)
	// Wrap in quotes to force phrase search
	return `"` + escaped + `"`
}

// SearchFTS performs a full-text search on classifications.
func (db *DB) SearchFTS(ctx context.Context, query string) ([]*Classification, error) {
	sanitized := sanitizeFTSQuery(query)

	rows, err := db.QueryContext(ctx, `
		SELECT c.id, c.document_id, c.source, c.model, c.tipo, c.tipo_original,
		       c.subject, c.subject_hierarchy, c.autor, c.titulo, c.anio,
		       c.editorial, c.edicion, c.description, c.confidence, c.accepted,
		       c.accepted_at, c.reason, c.created_at
		FROM classifications c
		JOIN classifications_fts fts ON c.id = fts.rowid
		WHERE classifications_fts MATCH ?
		ORDER BY rank`, sanitized)
	if err != nil {
		return nil, fmt.Errorf("searching FTS: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return scanClassifications(rows)
}

// GetStats returns summary statistics about the manifest.
func (db *DB) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{ByType: make(map[string]int)}

	// Document count
	var docCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
		return nil, fmt.Errorf("counting documents: %w", err)
	}
	stats.Documents = docCount

	// Classification count
	var classCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM classifications").Scan(&classCount); err != nil {
		return nil, fmt.Errorf("counting classifications: %w", err)
	}
	stats.Classifications = classCount

	// Accepted classifications
	var acceptedCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM classifications WHERE accepted = TRUE").Scan(&acceptedCount); err != nil {
		return nil, fmt.Errorf("counting accepted: %w", err)
	}
	stats.Accepted = acceptedCount

	// Documents by type
	rows, err := db.QueryContext(ctx, "SELECT file_type, COUNT(*) FROM documents GROUP BY file_type")
	if err != nil {
		return nil, fmt.Errorf("querying by type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var fileType string
		var count int
		if err := rows.Scan(&fileType, &count); err != nil {
			return nil, fmt.Errorf("scanning type: %w", err)
		}
		stats.ByType[fileType] = count
	}

	return stats, rows.Err()
}

// scanClassificationNullable scans nullable database fields into a classificationNullable struct.
func scanClassificationNullable(
	model, tipo, tipoOriginal, subject, subjectHierarchy,
	autor, titulo, anio, editorial, edicion, description, reason sql.NullString,
	acceptedAt sql.NullTime,
) classificationNullable {
	cn := classificationNullable{}

	if model.Valid {
		cn.Model = &model.String
	}
	if tipo.Valid {
		cn.Tipo = &tipo.String
	}
	if tipoOriginal.Valid {
		cn.TipoOriginal = &tipoOriginal.String
	}
	if subject.Valid {
		cn.Subject = &subject.String
	}
	if subjectHierarchy.Valid {
		cn.SubjectHierarchy = &subjectHierarchy.String
	}
	if autor.Valid {
		cn.Autor = &autor.String
	}
	if titulo.Valid {
		cn.Titulo = &titulo.String
	}
	if anio.Valid {
		cn.Anio = &anio.String
	}
	if editorial.Valid {
		cn.Editorial = &editorial.String
	}
	if edicion.Valid {
		cn.Edicion = &edicion.String
	}
	if description.Valid {
		cn.Description = &description.String
	}
	if reason.Valid {
		cn.Reason = &reason.String
	}
	if acceptedAt.Valid {
		cn.AcceptedAt = &acceptedAt.Time
	}

	return cn
}

// scanClassifications is a helper to scan multiple classification rows.
func scanClassifications(rows *sql.Rows) ([]*Classification, error) {
	var classes []*Classification
	for rows.Next() {
		c := &Classification{}
		var source string
		var model, tipo, tipoOriginal, subject, subjectHierarchy sql.NullString
		var autor, titulo, anio, editorial, edicion, description, reason sql.NullString
		var acceptedAt sql.NullTime

		if err := rows.Scan(
			&c.ID, &c.DocumentID, &source, &model, &tipo, &tipoOriginal,
			&subject, &subjectHierarchy, &autor, &titulo, &anio,
			&editorial, &edicion, &description, &c.Confidence, &c.Accepted,
			&acceptedAt, &reason, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning classification: %w", err)
		}

		c.Source = ClassSource(source)
		cn := scanClassificationNullable(model, tipo, tipoOriginal, subject, subjectHierarchy,
			autor, titulo, anio, editorial, edicion, description, reason, acceptedAt)
		c.Model = cn.Model
		c.Tipo = cn.Tipo
		c.TipoOriginal = cn.TipoOriginal
		c.Subject = cn.Subject
		c.SubjectHierarchy = cn.SubjectHierarchy
		c.Autor = cn.Autor
		c.Titulo = cn.Titulo
		c.Anio = cn.Anio
		c.Editorial = cn.Editorial
		c.Edicion = cn.Edicion
		c.Description = cn.Description
		c.Reason = cn.Reason
		c.AcceptedAt = cn.AcceptedAt

		classes = append(classes, c)
	}
	return classes, rows.Err()
}

// extractVersion extracts the version number from a migration filename.
// Example: "001_initial.sql" -> 1
func extractVersion(filename string) int {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) == 0 {
		return 0
	}
	v, _ := strconv.Atoi(parts[0])
	return v
}

// mkdirAll is a wrapper around os.MkdirAll.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}
