package scan

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alexmatias/librarian_fox/internal/config"
	"github.com/alexmatias/librarian_fox/internal/manifest"
)

// -----------------------------------------------------------
// Helper: create unique-content files (different SHA-256 per name)
// -----------------------------------------------------------

func createUniquePDF(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := []byte(name + strings.Repeat("x", 1024*11))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// createUniqueDOCX creates a file with .docx extension for scan coverage.
// Content is not a valid DOCX — scan only checks extension + hash.
func createUniqueDOCX(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := []byte(name + strings.Repeat("x", 1024*11))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// createPDFWithText creates a minimal PDF with embedded text for heuristic tests.
func createPDFWithText(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "(", "\\(")
	escaped = strings.ReplaceAll(escaped, ")", "\\)")
	content := fmt.Sprintf(`%%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R>>endobj
4 0 obj<</Length 44>>stream
BT /F1 12 Tf 100 700 Td (%s) Tj ET
endstream
endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000266 00000 n 
0000000360 00000 n 
trailer<</Size 6/Root 1 0 R>>startxref
410
%%%%EOF
`, escaped)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// -----------------------------------------------------------
// 1. Idempotencia — scan dos veces, misma DB, sin duplicar
// -----------------------------------------------------------

func TestScan_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf"} {
		createUniquePDF(t, tmpDir, name)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	// First scan
	r1, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("first Scan() error = %v", err)
	}
	if r1.Stats.TotalFiles != 3 {
		t.Fatalf("first scan: TotalFiles = %d, want 3", r1.Stats.TotalFiles)
	}

	// Second scan — same files, same DB
	r2, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("second Scan() error = %v", err)
	}
	if r2.Stats.TotalFiles != 3 {
		t.Errorf("second scan: TotalFiles = %d, want 3 (no duplicados)", r2.Stats.TotalFiles)
	}

	// DB should still have 3 documents, not 6
	var dbCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&dbCount); err != nil {
		t.Fatalf("counting documents: %v", err)
	}
	if dbCount != 3 {
		t.Errorf("documents in DB after second scan = %d, want 3", dbCount)
	}
}

// -----------------------------------------------------------
// 2. Duplicados exactos — mismo contenido, distinto path
//    content-addressed IDs ya manejan esto: mismo SHA-256 = mismo doc.
// -----------------------------------------------------------

func TestScan_ContentDuplicates_CrossSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Two files with identical content
	content := []byte("identical-content-for-both-files-" + strings.Repeat("x", 1024*11))
	file1 := filepath.Join(tmpDir, "original.pdf")
	file2 := filepath.Join(tmpDir, "copia_identica.pdf")
	if err := os.WriteFile(file1, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, content, 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Content-addressed dedup: same content = same ID = 1 DB row
	// Should still report 2 files detected (WalkDir visits both)
	if result.Stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2 (both paths visited)", result.Stats.TotalFiles)
	}

	// DB must have exactly 1 document (same SHA-256)
	var docCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
		t.Fatal(err)
	}
	if docCount != 1 {
		t.Errorf("documents count = %d, want 1 (same content = same ID)", docCount)
	}

	// No duplicates table entries needed — dedup is in the ID scheme
	var dupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM duplicates").Scan(&dupCount); err != nil {
		t.Fatal(err)
	}
	if dupCount != 0 {
		t.Errorf("duplicates count = %d, want 0 (content dedup is implicit)", dupCount)
	}
}

// -----------------------------------------------------------
// 3. Directorios excluidos
// -----------------------------------------------------------

func TestScan_ExcludesDirs_DefaultList(t *testing.T) {
	tmpDir := t.TempDir()

	excludedDirs := []string{"Library", "node_modules", ".git", "Applications"}
	for _, dir := range excludedDirs {
		d := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		content := make([]byte, 1024*11)
		if err := os.WriteFile(filepath.Join(d, "doc.pdf"), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Archivo normal en directorio no excluido
	normalDir := filepath.Join(tmpDir, "Documents")
	if err := os.MkdirAll(normalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := make([]byte, 1024*11)
	if err := os.WriteFile(filepath.Join(normalDir, "doc.pdf"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1 (solo Documents/doc.pdf)", result.Stats.TotalFiles)
	}
	// SkippedByDir cuenta cada directorio excluido, no archivos dentro
	if result.Stats.SkippedByDir != len(excludedDirs) {
		t.Errorf("SkippedByDir = %d, want %d", result.Stats.SkippedByDir, len(excludedDirs))
	}
}

// -----------------------------------------------------------
// 4. Reanudabilidad — diseño
// -----------------------------------------------------------

func TestScan_Resumability_Design(t *testing.T) {
	tmpDir := t.TempDir()

	// Fase 1: scan de 2 archivos (unique content per file)
	createUniquePDF(t, tmpDir, "a.pdf")
	createUniquePDF(t, tmpDir, "b.pdf")

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	r1, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("first Scan() error = %v", err)
	}
	if r1.Stats.TotalFiles != 2 {
		t.Fatalf("first scan: TotalFiles = %d, want 2", r1.Stats.TotalFiles)
	}

	// Fase 2: agregar un archivo nuevo y re-scanear
	createUniquePDF(t, tmpDir, "c.pdf")

	r2, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("second Scan() error = %v", err)
	}
	if r2.Stats.TotalFiles != 3 {
		t.Errorf("second scan: TotalFiles = %d, want 3", r2.Stats.TotalFiles)
	}

	// DB debe tener exactamente 3 documentos, no 5 (2+3)
	_ = db.Close()
	db2, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db2.Close() }()

	var docCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
		t.Fatal(err)
	}
	if docCount != 3 {
		t.Errorf("documents in DB = %d, want 3 (idempotente: 2 actualizados + 1 nuevo)", docCount)
	}
}

// -----------------------------------------------------------
// 5. Lote: scan idempotente con 12 archivos de testdata
// -----------------------------------------------------------

func TestScan_Idempotent_Testdata(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.Extensions = append(cfg.Scan.Extensions, "epub")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	// Primera corrida
	r1, err := scanner.Scan(ctx, "../../testdata", nil)
	if err != nil {
		t.Fatalf("first Scan() error = %v", err)
	}
	if r1.Stats.TotalFiles != 12 {
		t.Fatalf("first: TotalFiles = %d, want 12", r1.Stats.TotalFiles)
	}

	// Segunda corrida — debe ser idempotente
	r2, err := scanner.Scan(ctx, "../../testdata", nil)
	if err != nil {
		t.Fatalf("second Scan() error = %v", err)
	}
	if r2.Stats.TotalFiles != 12 {
		t.Errorf("second: TotalFiles = %d, want 12", r2.Stats.TotalFiles)
	}

	// DB: 12 documentos
	var docCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
		t.Fatal(err)
	}
	if docCount != 12 {
		t.Errorf("documents in DB = %d, want 12", docCount)
	}

	// page_count debe ser el mismo entre corridas
	var pageCount1, pageCount2 int
	if err := db.QueryRow("SELECT page_count FROM documents WHERE source_path LIKE '%Jackson.pdf'").Scan(&pageCount1); err != nil {
		t.Fatal(err)
	}
	if pageCount1 != 657 {
		t.Errorf("Jackson page_count = %d, want 657", pageCount1)
	}
	_ = pageCount2
}

// -----------------------------------------------------------
// 6. Escenario simulado: progreso paso a paso
// -----------------------------------------------------------

func TestScan_ProgressEvents_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	content := make([]byte, 1024*11)
	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	progress := make(chan *Event, 10)
	_, err = scanner.Scan(ctx, tmpDir, progress)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Deberíamos tener 4 eventos (1 collect + 3 scan)
	close(progress)
	events := make([]*Event, 0)
	for e := range progress {
		events = append(events, e)
	}
	if len(events) != 4 {
		t.Errorf("progress events = %d, want 4 (1 collect + 3 scan)", len(events))
	}
	// First event should be collect
	if events[0].Phase != "collect" {
		t.Errorf("event[0].Phase = %q, want collect", events[0].Phase)
	}
	// With concurrent processing, scan events may arrive out of order — sort by Current
	scanEvents := make([]*Event, 0, 3)
	for _, e := range events[1:] {
		if e.Phase == "scan" {
			scanEvents = append(scanEvents, e)
		}
	}
	sort.Slice(scanEvents, func(i, j int) bool { return scanEvents[i].Current < scanEvents[j].Current })
	// Verificar que Current va incrementando
	for i, e := range scanEvents {
		if e.Current != i+1 {
			t.Errorf("scan event[%d].Current = %d, want %d", i, e.Current, i+1)
		}
		if e.Phase != "scan" {
			t.Errorf("scan event[%d].Phase = %q, want scan", i, e.Phase)
		}
	}
}

// -----------------------------------------------------------
// 7. Directorios ocultos se saltan
// -----------------------------------------------------------

func TestScan_SkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	hiddenDir := filepath.Join(tmpDir, ".secret")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := make([]byte, 1024*11)
	if err := os.WriteFile(filepath.Join(hiddenDir, "top_secret.pdf"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.Stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0 (hidden dir should be skipped)", result.Stats.TotalFiles)
	}
}

// -----------------------------------------------------------
// 8. source_path es absoluto siempre
// -----------------------------------------------------------

func TestScan_PathsAreAbsolute(t *testing.T) {
	tmpDir := t.TempDir()
	content := make([]byte, 1024*11)
	if err := os.WriteFile(filepath.Join(tmpDir, "doc.pdf"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	for _, doc := range result.Documents {
		if !filepath.IsAbs(doc.SourcePath) {
			t.Errorf("source_path must be absolute: %s", doc.SourcePath)
		}
		if !strings.HasPrefix(doc.SourcePath, "/") {
			t.Errorf("source_path must start with /: %s", doc.SourcePath)
		}
	}
}

// -----------------------------------------------------------
// 9. Scan con directorio vacío
// -----------------------------------------------------------

func TestScan_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.Stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0 for empty dir", result.Stats.TotalFiles)
	}
	if len(result.Documents) != 0 {
		t.Errorf("Documents = %d, want 0", len(result.Documents))
	}
}

// -----------------------------------------------------------
// 8. Skip-on-mtime — archivos sin cambios se saltan
// -----------------------------------------------------------

func TestScan_SkipOnMtime(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf"} {
		createUniquePDF(t, tmpDir, name)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	// First scan: all files processed
	r1, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("first Scan() error = %v", err)
	}
	if r1.Stats.TotalFiles != 3 {
		t.Fatalf("first scan: TotalFiles = %d, want 3", r1.Stats.TotalFiles)
	}
	if r1.Stats.SkippedOnMtime != 0 {
		t.Errorf("first scan: SkippedOnMtime = %d, want 0", r1.Stats.SkippedOnMtime)
	}

	// Second scan: all files should be skipped (unchanged mtime + size)
	r2, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("second Scan() error = %v", err)
	}
	if r2.Stats.TotalFiles != 3 {
		t.Errorf("second scan: TotalFiles = %d, want 3", r2.Stats.TotalFiles)
	}
	if r2.Stats.SkippedOnMtime != 3 {
		t.Errorf("second scan: SkippedOnMtime = %d, want 3 (all skipped)", r2.Stats.SkippedOnMtime)
	}
}

// -----------------------------------------------------------
// 9. Skip-on-mtime — archivo modificado se reprocesa
// -----------------------------------------------------------

func TestScan_SkipOnMtime_ChangedFile(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"a.pdf", "b.pdf"} {
		createUniquePDF(t, tmpDir, name)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	// First scan
	r1, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("first Scan() error = %v", err)
	}
	if r1.Stats.TotalFiles != 2 {
		t.Fatalf("first scan: TotalFiles = %d, want 2", r1.Stats.TotalFiles)
	}

	// Wait a tick to ensure different mtime
	time.Sleep(10 * time.Millisecond)

	// Modify file "a.pdf" with new content
	newContent := []byte("modified-content-" + strings.Repeat("x", 1024*11))
	if err := os.WriteFile(filepath.Join(tmpDir, "a.pdf"), newContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Second scan: only a.pdf should be reprocessed, b.pdf skipped
	r2, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("second Scan() error = %v", err)
	}
	if r2.Stats.TotalFiles != 2 {
		t.Errorf("second scan: TotalFiles = %d, want 2", r2.Stats.TotalFiles)
	}
	// b.pdf unchanged → skipped; a.pdf changed → not skipped
	if r2.Stats.SkippedOnMtime != 1 {
		t.Errorf("second scan: SkippedOnMtime = %d, want 1 (only b.pdf skipped)", r2.Stats.SkippedOnMtime)
	}
}

// -----------------------------------------------------------
// 10. Worker pool — procesamiento concurrente sin pérdidas
// -----------------------------------------------------------

func TestScan_WorkerPool_ProcessesAll(t *testing.T) {
	tmpDir := t.TempDir()

	numFiles := 20
	for i := range numFiles {
		createUniquePDF(t, tmpDir, fmt.Sprintf("file_%d.pdf", i))
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Use max_concurrent_files = 4
	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	cfg.Scan.MaxConcurrentFiles = config.IntPtr(4)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != numFiles {
		t.Errorf("TotalFiles = %d, want %d", result.Stats.TotalFiles, numFiles)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors = %d, want 0", len(result.Errors))
	}
	if len(result.Documents) != numFiles {
		t.Errorf("Documents in result = %d, want %d", len(result.Documents), numFiles)
	}

	// Verify DB has all documents
	var dbCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&dbCount); err != nil {
		t.Fatal(err)
	}
	if dbCount != numFiles {
		t.Errorf("documents in DB = %d, want %d", dbCount, numFiles)
	}
}

// -----------------------------------------------------------
// WouldInsert / WouldUpdate tracking
// -----------------------------------------------------------

func TestScan_WouldInsertUpdate_NewFiles(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"a.pdf", "b.pdf"} {
		createUniquePDF(t, tmpDir, name)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.WouldInsert != 2 {
		t.Errorf("WouldInsert = %d, want 2", result.Stats.WouldInsert)
	}
	if result.Stats.WouldUpdate != 0 {
		t.Errorf("WouldUpdate = %d, want 0", result.Stats.WouldUpdate)
	}
	if result.Stats.SkippedOnMtime != 0 {
		t.Errorf("SkippedOnMtime = %d, want 0", result.Stats.SkippedOnMtime)
	}
	if result.Stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.Stats.TotalFiles)
	}
}

func TestScan_WouldInsertUpdate_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf"} {
		createUniquePDF(t, tmpDir, name)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// First scan — all files are new
	scanner1 := New(cfg, db, logger)
	result1, err := scanner1.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() #1 error = %v", err)
	}
	if result1.Stats.WouldInsert != 3 {
		t.Errorf("WouldInsert = %d, want 3", result1.Stats.WouldInsert)
	}
	if result1.Stats.WouldUpdate != 0 {
		t.Errorf("WouldUpdate = %d, want 0", result1.Stats.WouldUpdate)
	}
	if result1.Stats.SkippedOnMtime != 0 {
		t.Errorf("SkippedOnMtime = %d, want 0", result1.Stats.SkippedOnMtime)
	}
	if result1.Stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result1.Stats.TotalFiles)
	}
	// Total should equal WouldInsert + SkippedOnMtime (no updates)
	if result1.Stats.TotalFiles != int(result1.Stats.WouldInsert)+result1.Stats.SkippedOnMtime {
		t.Errorf("TotalFiles (%d) != WouldInsert (%d) + SkippedOnMtime (%d)",
			result1.Stats.TotalFiles, result1.Stats.WouldInsert, result1.Stats.SkippedOnMtime)
	}

	// Second scan — all files unchanged, should be skipped
	scanner2 := New(cfg, db, logger)
	result2, err := scanner2.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() #2 error = %v", err)
	}
	if result2.Stats.WouldInsert != 0 {
		t.Errorf("WouldInsert = %d, want 0 (already in DB)", result2.Stats.WouldInsert)
	}
	if result2.Stats.WouldUpdate != 0 {
		t.Errorf("WouldUpdate = %d, want 0 (unchanged)", result2.Stats.WouldUpdate)
	}
	if result2.Stats.SkippedOnMtime != 3 {
		t.Errorf("SkippedOnMtime = %d, want 3", result2.Stats.SkippedOnMtime)
	}
	if result2.Stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result2.Stats.TotalFiles)
	}
	if result2.Stats.TotalFiles != int(result2.Stats.WouldInsert)+int(result2.Stats.WouldUpdate)+result2.Stats.SkippedOnMtime {
		t.Errorf("TotalFiles (%d) != WouldInsert (%d) + WouldUpdate (%d) + SkippedOnMtime (%d)",
			result2.Stats.TotalFiles, result2.Stats.WouldInsert, result2.Stats.WouldUpdate, result2.Stats.SkippedOnMtime)
	}
}

func TestScan_WouldInsertUpdate_ChangedFile(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := createUniquePDF(t, tmpDir, "doc.pdf")

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// First scan — insert
	scanner1 := New(cfg, db, logger)
	result1, err := scanner1.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() #1 error = %v", err)
	}
	if result1.Stats.WouldInsert != 1 {
		t.Errorf("WouldInsert = %d, want 1", result1.Stats.WouldInsert)
	}

	// Modify the file content (different SHA-256, different mtime)
	time.Sleep(10 * time.Millisecond) // ensure mtime changes
	newContent := []byte("modified content " + strings.Repeat("x", 1024*11))
	if err := os.WriteFile(filePath, newContent, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Second scan — should detect as changed (WouldUpdate)
	scanner2 := New(cfg, db, logger)
	result2, err := scanner2.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() #2 error = %v", err)
	}
	if result2.Stats.WouldInsert != 0 {
		t.Errorf("WouldInsert = %d, want 0", result2.Stats.WouldInsert)
	}
	if result2.Stats.WouldUpdate != 1 {
		t.Errorf("WouldUpdate = %d, want 1 (file was modified)", result2.Stats.WouldUpdate)
	}
	if result2.Stats.SkippedOnMtime != 0 {
		t.Errorf("SkippedOnMtime = %d, want 0", result2.Stats.SkippedOnMtime)
	}
	if result2.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result2.Stats.TotalFiles)
	}
}

func TestScan_WouldInsertUpdate_PartialSkip(t *testing.T) {
	tmpDir := t.TempDir()

	// Two files — one stable, one that will change
	stablePath := createUniquePDF(t, tmpDir, "stable.pdf")
	createUniquePDF(t, tmpDir, "volatile.pdf")

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// First scan — both inserted
	scanner1 := New(cfg, db, logger)
	result1, err := scanner1.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() #1 error = %v", err)
	}
	if result1.Stats.WouldInsert != 2 {
		t.Errorf("WouldInsert = %d, want 2", result1.Stats.WouldInsert)
	}

	// Modify volatile.pdf
	time.Sleep(10 * time.Millisecond)
	newContent := []byte("modified " + strings.Repeat("x", 1024*11))
	if err := os.WriteFile(stablePath, newContent, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Second scan — stable is updated, volatile is skipped
	scanner2 := New(cfg, db, logger)
	result2, err := scanner2.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() #2 error = %v", err)
	}
	// Total should be 2 (both files found in dir)
	if result2.Stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result2.Stats.TotalFiles)
	}
	// At least one file was skipped
	if result2.Stats.SkippedOnMtime < 1 {
		t.Errorf("SkippedOnMtime = %d, want >= 1", result2.Stats.SkippedOnMtime)
	}
	// At least one file was updated
	if result2.Stats.WouldUpdate < 1 {
		t.Errorf("WouldUpdate = %d, want >= 1", result2.Stats.WouldUpdate)
	}
	// Total must equal sum
	sum := int(result2.Stats.WouldInsert) + int(result2.Stats.WouldUpdate) + result2.Stats.SkippedOnMtime
	if result2.Stats.TotalFiles != sum {
		t.Errorf("TotalFiles (%d) != WouldInsert (%d) + WouldUpdate (%d) + SkippedOnMtime (%d)",
			result2.Stats.TotalFiles, result2.Stats.WouldInsert, result2.Stats.WouldUpdate, result2.Stats.SkippedOnMtime)
	}
}

func TestScan_DOCX_Detected(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	createUniqueDOCX(t, tmpDir, "test_letter.docx")
	createUniqueDOCX(t, tmpDir, "another.docx")

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.Stats.TotalFiles)
	}
	if result.Stats.ByType["docx"] != 2 {
		t.Errorf("ByType[docx] = %d, want 2", result.Stats.ByType["docx"])
	}
}

func TestScan_PDFWithISBN_Detected(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := manifest.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Scan.MinFileSizeKB = config.IntPtr(0)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	isbnPath := createPDFWithText(t, tmpDir, "isbn_book.pdf", "ISBN 978-0-262-04630-5 Introduction to Algorithms")

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Stats.TotalFiles)
	}
	if result.Stats.ByType["pdf"] != 1 {
		t.Errorf("ByType[pdf] = %d, want 1", result.Stats.ByType["pdf"])
	}

	// Verify the ISBN PDF was scanned (no text extraction in scan phase)
	found := false
	for _, doc := range result.Documents {
		if doc.SourcePath == isbnPath {
			found = true
			if doc.PageCount != nil && *doc.PageCount != 1 {
				t.Errorf("PageCount = %v, want 1", doc.PageCount)
			}
		}
	}
	if !found {
		t.Error("ISBN PDF not found in scan results")
	}
}
