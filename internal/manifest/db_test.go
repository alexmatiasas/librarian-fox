package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should exist")
	}

	// Verify WAL mode
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode error = %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	// Verify foreign keys
	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("PRAGMA foreign_keys error = %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Run migrations
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Verify tables exist
	tables := []string{
		"documents", "known_types", "subject_groups", "subject_aliases",
		"classifications", "classifications_fts", "tags", "document_tags",
		"organization", "duplicates", "_migrations",
	}

	for _, table := range tables {
		var count int
		query := "SELECT COUNT(*) FROM " + table
		if err := db.QueryRow(query).Scan(&count); err != nil {
			t.Errorf("Table %q should exist: %v", table, err)
		}
	}

	// Verify known_types were inserted
	var typeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM known_types").Scan(&typeCount); err != nil {
		t.Fatalf("counting known_types: %v", err)
	}
	if typeCount != 8 {
		t.Errorf("known_types count = %d, want 8", typeCount)
	}

	// Verify subject_groups were inserted
	var groupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM subject_groups").Scan(&groupCount); err != nil {
		t.Fatalf("counting subject_groups: %v", err)
	}
	if groupCount != 7 {
		t.Errorf("subject_groups count = %d, want 7", groupCount)
	}

	// Verify tags were inserted
	var tagCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&tagCount); err != nil {
		t.Fatalf("counting tags: %v", err)
	}
	if tagCount != 9 {
		t.Errorf("tags count = %d, want 9", tagCount)
	}

	// Verify migration version
	var version int
	if err := db.QueryRow("SELECT MAX(version) FROM _migrations").Scan(&version); err != nil {
		t.Fatalf("getting migration version: %v", err)
	}
	if version != 1 {
		t.Errorf("migration version = %d, want 1", version)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Run migrations twice
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() first run error = %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() second run error = %v", err)
	}
}

func TestInsertAndGetDocument(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	doc := &Document{
		ID:           "abc123def456",
		SourcePath:   "/Users/test/Documents/test.pdf",
		FileType:     "pdf",
		FileSize:     1024000,
		HasTextLayer: true,
		NeedsOCR:     false,
	}

	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() error = %v", err)
	}

	got, err := db.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}

	if got.ID != doc.ID {
		t.Errorf("ID = %q, want %q", got.ID, doc.ID)
	}
	if got.SourcePath != doc.SourcePath {
		t.Errorf("SourcePath = %q, want %q", got.SourcePath, doc.SourcePath)
	}
	if got.FileType != doc.FileType {
		t.Errorf("FileType = %q, want %q", got.FileType, doc.FileType)
	}
	if got.FileSize != doc.FileSize {
		t.Errorf("FileSize = %d, want %d", got.FileSize, doc.FileSize)
	}
	if got.HasTextLayer != doc.HasTextLayer {
		t.Errorf("HasTextLayer = %v, want %v", got.HasTextLayer, doc.HasTextLayer)
	}
}

func TestListDocuments(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Insert multiple documents
	docs := []*Document{
		{ID: "doc1", SourcePath: "/path/1.pdf", FileType: "pdf", FileSize: 1000},
		{ID: "doc2", SourcePath: "/path/2.pdf", FileType: "pdf", FileSize: 2000},
		{ID: "doc3", SourcePath: "/path/3.epub", FileType: "epub", FileSize: 500},
	}

	for _, doc := range docs {
		if err := db.InsertDocument(ctx, doc); err != nil {
			t.Fatalf("InsertDocument(%s) error = %v", doc.ID, err)
		}
	}

	list, err := db.ListDocuments(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}

	if len(list) != 3 {
		t.Errorf("ListDocuments() returned %d documents, want 3", len(list))
	}
}

func TestInsertAndGetClassification(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Insert document first
	doc := &Document{
		ID:         "test-doc-id",
		SourcePath: "/path/test.pdf",
		FileType:   "pdf",
		FileSize:   1000,
	}
	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() error = %v", err)
	}

	// Insert classification
	class := &Classification{
		DocumentID: "test-doc-id",
		Source:     "llm",
		Model:      strPtr("qwen3:8b"),
		Tipo:       strPtr("Paper"),
		Subject:    strPtr("Computer Science"),
		Autor:      strPtr("Test Author"),
		Titulo:     strPtr("Test Title"),
		Confidence: 0.85,
		Accepted:   false,
	}

	if err := db.InsertClassification(ctx, class); err != nil {
		t.Fatalf("InsertClassification() error = %v", err)
	}

	if class.ID == 0 {
		t.Error("Classification ID should be set after insert")
	}
}

func TestAcceptClassification(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Insert document
	doc := &Document{
		ID:         "test-doc-id",
		SourcePath: "/path/test.pdf",
		FileType:   "pdf",
		FileSize:   1000,
	}
	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() error = %v", err)
	}

	// Insert two classifications
	class1 := &Classification{
		DocumentID: "test-doc-id",
		Source:     "llm",
		Tipo:       strPtr("Paper"),
		Confidence: 0.7,
	}
	class2 := &Classification{
		DocumentID: "test-doc-id",
		Source:     "llm",
		Tipo:       strPtr("Libro"),
		Confidence: 0.9,
	}

	if err := db.InsertClassification(ctx, class1); err != nil {
		t.Fatalf("InsertClassification(1) error = %v", err)
	}
	if err := db.InsertClassification(ctx, class2); err != nil {
		t.Fatalf("InsertClassification(2) error = %v", err)
	}

	// Accept first classification
	if err := db.AcceptClassification(ctx, class1.ID); err != nil {
		t.Fatalf("AcceptClassification() error = %v", err)
	}

	// Verify first is accepted
	got, err := db.GetAcceptedClassification(ctx, "test-doc-id")
	if err != nil {
		t.Fatalf("GetAcceptedClassification() error = %v", err)
	}
	if got.ID != class1.ID {
		t.Errorf("Accepted classification ID = %d, want %d", got.ID, class1.ID)
	}

	// Accept second classification (should unaccept first)
	if err := db.AcceptClassification(ctx, class2.ID); err != nil {
		t.Fatalf("AcceptClassification() error = %v", err)
	}

	got, err = db.GetAcceptedClassification(ctx, "test-doc-id")
	if err != nil {
		t.Fatalf("GetAcceptedClassification() error = %v", err)
	}
	if got.ID != class2.ID {
		t.Errorf("Accepted classification ID = %d, want %d", got.ID, class2.ID)
	}
}

func TestUpsertSubjectGroup(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Create new group
	group, err := db.UpsertSubjectGroup(ctx, "Quantum Physics")
	if err != nil {
		t.Fatalf("UpsertSubjectGroup() error = %v", err)
	}
	if group.Name != "Quantum Physics" {
		t.Errorf("Group name = %q, want %q", group.Name, "Quantum Physics")
	}

	// Get existing group
	group2, err := db.UpsertSubjectGroup(ctx, "Quantum Physics")
	if err != nil {
		t.Fatalf("UpsertSubjectGroup() second call error = %v", err)
	}
	if group.ID != group2.ID {
		t.Errorf("Same name should return same group, got IDs %d and %d", group.ID, group2.ID)
	}
}

func TestGetStats(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Insert some documents
	for i := 0; i < 5; i++ {
		doc := &Document{
			ID:         "doc" + string(rune('0'+i)),
			SourcePath: "/path/file" + string(rune('0'+i)) + ".pdf",
			FileType:   "pdf",
			FileSize:   1000,
		}
		if err := db.InsertDocument(ctx, doc); err != nil {
			t.Fatalf("InsertDocument() error = %v", err)
		}
	}

	stats, err := db.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.Documents != 5 {
		t.Errorf("Documents = %d, want 5", stats.Documents)
	}
}

func TestSearchFTS(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Insert document
	doc := &Document{
		ID:         "test-doc-id",
		SourcePath: "/path/test.pdf",
		FileType:   "pdf",
		FileSize:   1000,
	}
	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() error = %v", err)
	}

	// Insert classification with searchable text
	class := &Classification{
		DocumentID: "test-doc-id",
		Source:     "llm",
		Titulo:     strPtr("Introduction to Quantum Computing"),
		Autor:      strPtr("Jane Smith"),
		Subject:    strPtr("Physics"),
		Confidence: 0.9,
	}
	if err := db.InsertClassification(ctx, class); err != nil {
		t.Fatalf("InsertClassification() error = %v", err)
	}

	// Search for "Quantum"
	results, err := db.SearchFTS(ctx, "Quantum")
	if err != nil {
		t.Fatalf("SearchFTS() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("SearchFTS() returned %d results, want 1", len(results))
	}
	if results[0].ID != class.ID {
		t.Errorf("Search result ID = %d, want %d", results[0].ID, class.ID)
	}
}

func TestRecordOrganization(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Insert document
	doc := &Document{
		ID:         "test-doc-id",
		SourcePath: "/path/test.pdf",
		FileType:   "pdf",
		FileSize:   1000,
	}
	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() error = %v", err)
	}

	// Record organization
	org := &Organization{
		DocumentID: "test-doc-id",
		SourcePath: "/path/test.pdf",
		TargetPath: "/Library/Papers/test.pdf",
		BackupPath: strPtr("/Library/.backups/test.pdf"),
	}

	if err := db.RecordOrganization(ctx, org); err != nil {
		t.Fatalf("RecordOrganization() error = %v", err)
	}
}

func TestMarkDuplicate(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Insert two documents
	doc1 := &Document{ID: "doc1", SourcePath: "/path/1.pdf", FileType: "pdf", FileSize: 1000}
	doc2 := &Document{ID: "doc2", SourcePath: "/path/2.pdf", FileType: "pdf", FileSize: 1000}

	if err := db.InsertDocument(ctx, doc1); err != nil {
		t.Fatalf("InsertDocument(1) error = %v", err)
	}
	if err := db.InsertDocument(ctx, doc2); err != nil {
		t.Fatalf("InsertDocument(2) error = %v", err)
	}

	// Mark as duplicate
	dup := &Duplicate{
		DocumentID:  "doc2",
		DuplicateOf: "doc1",
		Similarity:  0.95,
		Reason:      "same_content",
	}

	if err := db.MarkDuplicate(ctx, dup); err != nil {
		t.Fatalf("MarkDuplicate() error = %v", err)
	}
}

// openTestDB creates an in-memory test database with migrations applied.
func openTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}

func strPtr(s string) *string {
	return &s
}

func TestOpen_ConnectionPool(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	stats := db.Stats()

	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", stats.MaxOpenConnections)
	}

	// MaxIdleConns is not exposed in DBStats, but we can verify
	// the connection works correctly by checking Idle count
	if stats.Idle < 0 {
		t.Errorf("Idle connections should be >= 0, got %d", stats.Idle)
	}
}

func TestOpen_InMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) error = %v", err)
	}
	defer func() { _ = db.Close() }()

	// Verify it works by running a query
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master").Scan(&count); err != nil {
		t.Fatalf("Query error = %v", err)
	}

	// If we got here, the in-memory DB works
}

func TestOpen_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "test.db")

	db, err := Open(nestedPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Errorf("Database file should exist at %s", nestedPath)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	// Try to create a DB in a read-only directory
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o555); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	defer func() {
		_ = os.Chmod(readOnlyDir, 0o755) // restore permissions for cleanup
	}()

	invalidPath := filepath.Join(readOnlyDir, "test.db")

	_, err := Open(invalidPath)
	if err == nil {
		t.Error("Open() should fail with read-only directory")
	}
}

func TestInsertDocument_OnConflict(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	doc := &Document{
		ID:         "test-doc",
		SourcePath: "/path/original.pdf",
		FileType:   "pdf",
		FileSize:   1000,
	}

	// Insert first time
	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() first insert error = %v", err)
	}

	// Get created_at
	got1, err := db.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	createdAt := got1.CreatedAt

	// Update with different path
	doc.SourcePath = "/path/updated.pdf"
	doc.FileSize = 2000

	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatalf("InsertDocument() second insert error = %v", err)
	}

	// Verify update worked
	got2, err := db.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}

	if got2.SourcePath != "/path/updated.pdf" {
		t.Errorf("SourcePath = %q, want %q", got2.SourcePath, "/path/updated.pdf")
	}
	if got2.FileSize != 2000 {
		t.Errorf("FileSize = %d, want 2000", got2.FileSize)
	}
	// created_at should be preserved (not reset)
	if got2.CreatedAt != createdAt {
		t.Errorf("CreatedAt changed from %v to %v, should be preserved", createdAt, got2.CreatedAt)
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	_, err := db.GetDocument(ctx, "nonexistent-id")
	if err == nil {
		t.Error("GetDocument() should fail for nonexistent document")
	}

	// Verify error message is clear
	expected := "document not found: nonexistent-id"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestListDocuments_Pagination(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Insert 5 documents
	for i := 0; i < 5; i++ {
		doc := &Document{
			ID:         "doc" + string(rune('0'+i)),
			SourcePath: "/path/file" + string(rune('0'+i)) + ".pdf",
			FileType:   "pdf",
			FileSize:   1000,
		}
		if err := db.InsertDocument(ctx, doc); err != nil {
			t.Fatalf("InsertDocument() error = %v", err)
		}
	}

	// Get all
	all, err := db.ListDocuments(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListDocuments(0, 0) error = %v", err)
	}
	if len(all) != 5 {
		t.Errorf("ListDocuments(0, 0) returned %d, want 5", len(all))
	}

	// Get first 2
	page1, err := db.ListDocuments(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListDocuments(2, 0) error = %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("ListDocuments(2, 0) returned %d, want 2", len(page1))
	}

	// Get next 2
	page2, err := db.ListDocuments(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListDocuments(2, 2) error = %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("ListDocuments(2, 2) returned %d, want 2", len(page2))
	}

	// Get last 1
	page3, err := db.ListDocuments(ctx, 2, 4)
	if err != nil {
		t.Fatalf("ListDocuments(2, 4) error = %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("ListDocuments(2, 4) returned %d, want 1", len(page3))
	}

	// Verify no overlap
	if page1[0].ID == page2[0].ID {
		t.Error("page1 and page2 should not overlap")
	}
}

func TestAcceptClassification_NotFound(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	err := db.AcceptClassification(ctx, 99999)
	if err == nil {
		t.Error("AcceptClassification() should fail for nonexistent classification")
	}

	// Verify error message is clear
	expected := "classification not found: 99999"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestMarkDuplicate_AlreadyExists(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Insert two documents
	doc1 := &Document{ID: "doc1", SourcePath: "/path/1.pdf", FileType: "pdf", FileSize: 1000}
	doc2 := &Document{ID: "doc2", SourcePath: "/path/2.pdf", FileType: "pdf", FileSize: 1000}

	if err := db.InsertDocument(ctx, doc1); err != nil {
		t.Fatalf("InsertDocument(1) error = %v", err)
	}
	if err := db.InsertDocument(ctx, doc2); err != nil {
		t.Fatalf("InsertDocument(2) error = %v", err)
	}

	// Mark as duplicate first time
	dup := &Duplicate{
		DocumentID:  "doc2",
		DuplicateOf: "doc1",
		Similarity:  0.95,
		Reason:      "same_content",
	}

	if err := db.MarkDuplicate(ctx, dup); err != nil {
		t.Fatalf("MarkDuplicate() first call error = %v", err)
	}

	// Mark same duplicate again (should not error)
	dup2 := &Duplicate{
		DocumentID:  "doc2",
		DuplicateOf: "doc1",
		Similarity:  0.95,
		Reason:      "same_content",
	}

	if err := db.MarkDuplicate(ctx, dup2); err != nil {
		t.Fatalf("MarkDuplicate() second call error = %v", err)
	}
}

func TestGetAcceptedClassification_NotFound(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	_, err := db.GetAcceptedClassification(ctx, "nonexistent-doc")
	if err == nil {
		t.Error("GetAcceptedClassification() should fail for nonexistent document")
	}

	// Verify error message is clear
	expected := "no accepted classification for document: nonexistent-doc"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestSanitizeFTSQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"quantum", `"quantum"`},
		{"quantum computing", `"quantum computing"`},
		{`some "query"`, `"some ""query"""`},
		{"* OR 1=1", `"* OR 1=1"`},
		{"", `""`},
	}

	for _, tt := range tests {
		got := sanitizeFTSQuery(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
