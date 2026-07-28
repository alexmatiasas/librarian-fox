package scan

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexmatias/librarian_fox/internal/config"
	"github.com/alexmatias/librarian_fox/internal/manifest"
)

func TestScan_Testdata(t *testing.T) {
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
	cfg.Scan.Extensions = append(cfg.Scan.Extensions, "epub")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)

	result, err := scanner.Scan(ctx, "../../testdata", nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// All paths must be absolute
	for _, doc := range result.Documents {
		if !filepath.IsAbs(doc.SourcePath) {
			t.Errorf("source_path must be absolute: %s", doc.SourcePath)
		}
	}

	// Verify counts
	if result.Stats.TotalFiles != 12 {
		t.Errorf("TotalFiles = %d, want 12", result.Stats.TotalFiles)
	}
	if result.Stats.ByType["pdf"] != 9 {
		t.Errorf("ByType[pdf] = %d, want 9", result.Stats.ByType["pdf"])
	}
	if result.Stats.ByType["djvu"] != 2 {
		t.Errorf("ByType[djvu] = %d, want 2", result.Stats.ByType["djvu"])
	}
	if result.Stats.ByType["epub"] != 1 {
		t.Errorf("ByType[epub] = %d, want 1", result.Stats.ByType["epub"])
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors = %d, want 0", len(result.Errors))
	}

	// Verify page_count for known PDFs
	for _, doc := range result.Documents {
		switch filepath.Base(doc.SourcePath) {
		case "Classical Electrodynamics - Jackson.pdf":
			if doc.PageCount == nil || *doc.PageCount != 657 {
				t.Errorf("Jackson PDF page_count = %v, want 657", doc.PageCount)
			}
			if doc.HasTextLayer {
				t.Errorf("Jackson PDF has_text_layer = true, want false (scanned)")
			}
			if !doc.NeedsOCR {
				t.Errorf("Jackson PDF needs_ocr = false, want true")
			}
		case "Hands-On_Machine_Learning_with_Scikit-Learn-Keras-and-TensorFlow-2nd-Edition-Aurelien-Geron.pdf":
			if doc.PageCount == nil || *doc.PageCount != 851 {
				t.Errorf("ML book page_count = %v, want 851", doc.PageCount)
			}
			if !doc.HasTextLayer {
				t.Errorf("ML book has_text_layer = false, want true")
			}
			if doc.NeedsOCR {
				t.Errorf("ML book needs_ocr = true, want false")
			}
		case "Tesis.pdf":
			if doc.PageCount == nil || *doc.PageCount != 109 {
				t.Errorf("Tesis page_count = %v, want 109", doc.PageCount)
			}
			if !doc.HasTextLayer {
				t.Errorf("Tesis has_text_layer = false, want true")
			}
		case "EjerciciosParticulas.pdf":
			if doc.PageCount == nil || *doc.PageCount != 9 {
				t.Errorf("Ejercicios page_count = %v, want 9", doc.PageCount)
			}
		case "qft3.pdf":
			if doc.PageCount == nil || *doc.PageCount != 2 {
				t.Errorf("qft3 page_count = %v, want 2", doc.PageCount)
			}
		case "56406005-Jackson-Solutions.pdf":
			if doc.PageCount == nil || *doc.PageCount != 412 {
				t.Errorf("Jackson Solutions page_count = %v, want 412", doc.PageCount)
			}
		case "Tan et al. - 2019 - Some characteristic parameters of proton from the bag model.pdf":
			if doc.PageCount == nil || *doc.PageCount != 4 {
				t.Errorf("Tan et al. page_count = %v, want 4", doc.PageCount)
			}
		case "John David Jackson - Classical electrodynamics-Wiley (1999).djvu":
			if doc.PageCount == nil || *doc.PageCount != 833 {
				t.Errorf("Jackson DjVu page_count = %v, want 833", doc.PageCount)
			}
		case "(Cambridge Mathematical Library) G. K. Batchelor - An Introduction to Fluid Dynamics -Cambridge University Press (2000).djvu":
			// DjVu page count for this file — just verify it's > 0
			if doc.PageCount == nil || *doc.PageCount == 0 {
				t.Errorf("Batchelor DjVu page_count = %v, want > 0", doc.PageCount)
			}
		}
	}

	// Verify duplicates table is empty (no content duplicates in testdata)
	var dupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM duplicates").Scan(&dupCount); err != nil {
		t.Fatalf("counting duplicates: %v", err)
	}
	if dupCount != 0 {
		t.Errorf("duplicates count = %d, want 0", dupCount)
	}
}

func TestScan_ContentDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Two identical files (same content = same SHA-256)
	content := make([]byte, 1024*11) // 11KB
	file1 := filepath.Join(tmpDir, "original.pdf")
	if err := os.WriteFile(file1, content, 0o644); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(tmpDir, "copy.pdf")
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should find 2 files (same content = same SHA-256 = 1 DB row,
	// but both paths are visited by WalkDir)
	if result.Stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.Stats.TotalFiles)
	}

	// Content-addressed dedup: same content → same ID → only 1 DB row
	var docCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
		t.Fatalf("counting documents: %v", err)
	}
	if docCount != 1 {
		t.Errorf("documents in DB = %d, want 1 (same content = same ID)", docCount)
	}

	// No duplicates table needed — dedup is intrinsic to the ID scheme
	var dupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM duplicates").Scan(&dupCount); err != nil {
		t.Fatalf("counting duplicates: %v", err)
	}
	if dupCount != 0 {
		t.Errorf("duplicates in DB = %d, want 0", dupCount)
	}
}

func TestScan_GetPageCount_PDF(t *testing.T) {
	// Test with a known PDF from testdata
	path := "../../testdata/Libros/Physics/Classical Electrodynamics - Jackson.pdf"
	count := getPageCount(path, manifest.FileTypePDF)
	if count == nil {
		t.Fatal("getPageCount returned nil")
	}
	if *count != 657 {
		t.Errorf("page count = %d, want 657", *count)
	}
}

func TestScan_GetPageCount_DjVu(t *testing.T) {
	path := "../../testdata/Libros/Physics/John David Jackson - Classical electrodynamics-Wiley (1999).djvu"
	count := getPageCount(path, manifest.FileTypeDjVu)
	if count == nil {
		t.Fatal("getPageCount returned nil for DjVu")
	}
	if *count != 833 {
		t.Errorf("page count = %d, want 833", *count)
	}
}

func TestScan_GetPageCount_UnknownType(t *testing.T) {
	// EPUB should return nil (no page count tool)
	count := getPageCount("/fake/file.epub", manifest.FileTypeEPUB)
	if count != nil {
		t.Errorf("EPUB page count = %v, want nil", *count)
	}
}

func TestScan_HasTextLayer_ScannedPDF(t *testing.T) {
	// Jackson PDF is an image-based scan — no text layer
	path := "../../testdata/Libros/Physics/Classical Electrodynamics - Jackson.pdf"
	if hasTextLayer(path, manifest.FileTypePDF) {
		t.Error("Scanned PDF hasTextLayer = true, want false")
	}
}

func TestScan_HasTextLayer_TextPDF(t *testing.T) {
	// Tesis PDF has extractable text
	path := "../../testdata/Tesis/Physics/Tesis.pdf"
	if !hasTextLayer(path, manifest.FileTypePDF) {
		t.Error("Tesis hasTextLayer = false, want true")
	}
}

func TestScan_HasTextLayer_EPUB(t *testing.T) {
	// EPUB always returns true (assumed to have text)
	if !hasTextLayer("/fake/file.epub", manifest.FileTypeEPUB) {
		t.Error("EPUB hasTextLayer = false, want true")
	}
}

func TestScan_AbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := "mysubdir"
	fullSubDir := filepath.Join(tmpDir, subDir)
	if err := os.MkdirAll(fullSubDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pdfContent := make([]byte, 1024*11)
	pdfFile := filepath.Join(fullSubDir, "doc.pdf")
	if err := os.WriteFile(pdfFile, pdfContent, 0o644); err != nil {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Change cwd to tmpDir so we can pass a relative root
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	scanner := New(cfg, db, logger)
	// Pass relative path "mysubdir" — scanner should resolve to absolute
	result, err := scanner.Scan(ctx, subDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	for _, doc := range result.Documents {
		if !filepath.IsAbs(doc.SourcePath) {
			t.Errorf("source_path must be absolute, got: %s", doc.SourcePath)
		}
		if !strings.Contains(doc.SourcePath, subDir) {
			t.Errorf("source_path should contain %q, got: %s", subDir, doc.SourcePath)
		}
		if !strings.HasSuffix(doc.SourcePath, "doc.pdf") {
			t.Errorf("source_path should end with doc.pdf, got: %s", doc.SourcePath)
		}
	}
}

func TestScan_FiltersByExtension(t *testing.T) {
	tmpDir := t.TempDir()

	txtFile := filepath.Join(tmpDir, "readme.txt")
	if err := os.WriteFile(txtFile, []byte("hello world this is a test file with enough content to pass size filter"), 0o644); err != nil {
		t.Fatal(err)
	}

	pdfContent := make([]byte, 1024*11)
	pdfFile := filepath.Join(tmpDir, "document.pdf")
	if err := os.WriteFile(pdfFile, pdfContent, 0o644); err != nil {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Stats.TotalFiles)
	}
}

func TestScan_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	realContent := make([]byte, 1024*11)
	realFile := filepath.Join(tmpDir, "real.pdf")
	if err := os.WriteFile(realFile, realContent, 0o644); err != nil {
		t.Fatal(err)
	}

	linkFile := filepath.Join(tmpDir, "link.pdf")
	if err := os.Symlink(realFile, linkFile); err != nil {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Stats.TotalFiles)
	}
	if result.Stats.SymlinksSkipped != 1 {
		t.Errorf("SymlinksSkipped = %d, want 1", result.Stats.SymlinksSkipped)
	}
}

func TestScan_ExcludesDirs(t *testing.T) {
	tmpDir := t.TempDir()

	excludedDir := filepath.Join(tmpDir, "Library")
	if err := os.MkdirAll(excludedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	excludedContent := make([]byte, 1024*11)
	excludedFile := filepath.Join(excludedDir, "excluded.pdf")
	if err := os.WriteFile(excludedFile, excludedContent, 0o644); err != nil {
		t.Fatal(err)
	}

	normalDir := filepath.Join(tmpDir, "Documents")
	if err := os.MkdirAll(normalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	normalContent := make([]byte, 1024*11)
	normalFile := filepath.Join(normalDir, "document.pdf")
	if err := os.WriteFile(normalFile, normalContent, 0o644); err != nil {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Stats.TotalFiles)
	}
	if result.Stats.SkippedByDir != 1 {
		t.Errorf("SkippedByDir = %d, want 1", result.Stats.SkippedByDir)
	}
}

func TestScan_RespectsMaxDepth(t *testing.T) {
	tmpDir := t.TempDir()

	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deepContent := make([]byte, 1024*11)
	deepFile := filepath.Join(deepDir, "deep.pdf")
	if err := os.WriteFile(deepFile, deepContent, 0o644); err != nil {
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
	cfg.Scan.MaxDepth = config.IntPtr(2)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", result.Stats.TotalFiles)
	}
}

func TestScan_FiltersBySize(t *testing.T) {
	tmpDir := t.TempDir()

	smallFile := filepath.Join(tmpDir, "small.pdf")
	if err := os.WriteFile(smallFile, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	normalFile := filepath.Join(tmpDir, "normal.pdf")
	if err := os.WriteFile(normalFile, make([]byte, 1024*1024), 0o644); err != nil {
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
	cfg.Scan.MinFileSizeKB = config.IntPtr(10)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)
	result, err := scanner.Scan(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Stats.TotalFiles)
	}
	if result.Stats.SkippedBySize != 1 {
		t.Errorf("SkippedBySize = %d, want 1", result.Stats.SkippedBySize)
	}
}

func TestScan_ProgressEvents(t *testing.T) {
	tmpDir := t.TempDir()

	pdfContent := make([]byte, 1024*11)
	pdfFile := filepath.Join(tmpDir, "document.pdf")
	if err := os.WriteFile(pdfFile, pdfContent, 0o644); err != nil {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	scanner := New(cfg, db, logger)

	progress := make(chan *Event, 10)
	_, err = scanner.Scan(ctx, tmpDir, progress)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(progress) != 2 {
		t.Errorf("progress events = %d, want 2 (collect + scan)", len(progress))
	}

	// First event should be collect phase
	event1 := <-progress
	if event1.Phase != "collect" {
		t.Errorf("event[0].Phase = %q, want collect", event1.Phase)
	}

	// Last event should be scan phase
	event2 := <-progress
	if event2.Phase != "scan" {
		t.Errorf("event[1].Phase = %q, want scan", event2.Phase)
	}
	if event2.Current != 1 {
		t.Errorf("event[1].Current = %d, want 1", event2.Current)
	}
}
