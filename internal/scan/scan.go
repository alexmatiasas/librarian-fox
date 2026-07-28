package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexmatias/librarian_fox/internal/config"
	"github.com/alexmatias/librarian_fox/internal/manifest"
)

// Event represents a progress event during scanning.
type Event struct {
	Phase   string
	Current int
	Total   int
	Message string
}

// Scanner discovers documents in a directory.
type Scanner struct {
	config   *config.Config
	manifest *manifest.DB
	logger   *slog.Logger
}

// Result contains the outcome of a scan operation.
type Result struct {
	Documents []manifest.Document
	Stats     Stats
	Errors    []ScanError
}

// Stats holds summary statistics from a scan.
type Stats struct {
	TotalFiles      int
	ByType          map[string]int
	TotalSize       int64
	SkippedBySize   int
	SkippedByDir    int
	SymlinksSkipped int
	WithTextLayer   int
	NeedingOCR      int
}

// ScanError records a file that could not be processed.
type ScanError struct {
	Path  string
	Error error
}

// New creates a Scanner with the given configuration.
func New(cfg *config.Config, db *manifest.DB, logger *slog.Logger) *Scanner {
	return &Scanner{
		config:   cfg,
		manifest: db,
		logger:   logger,
	}
}

// Scan traverses the root directory and populates the manifest.
// It returns a Result with all discovered documents and statistics.
// progress is an optional channel for receiving progress events.
func (s *Scanner) Scan(ctx context.Context, root string, progress chan<- *Event) (*Result, error) {
	result := &Result{
		Stats: Stats{ByType: make(map[string]int)},
	}

	// Expand root path (handle ~) and resolve to absolute
	root = expandTilde(root)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}
	root = absRoot

	// Verify root exists
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("cannot access root directory %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path %s is not a directory", root)
	}

	s.logger.Info("starting scan", "root", root)

	// Walk directory tree
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.logger.Warn("cannot access path", "path", path, "error", err)
			return nil // skip unreadable paths
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get file info
		fi, err := d.Info()
		if err != nil {
			s.logger.Warn("cannot get file info", "path", path, "error", err)
			return nil
		}

		// Skip symlinks
		if fi.Mode()&os.ModeSymlink != 0 {
			result.Stats.SymlinksSkipped++
			s.logger.Debug("skipping symlink", "path", path)
			return nil
		}

		// Handle directories
		if d.IsDir() {
			if s.shouldExcludeDir(path, root) {
				result.Stats.SkippedByDir++
				s.logger.Debug("skipping excluded dir", "path", path)
				return filepath.SkipDir
			}
			return nil
		}

		// Check depth
		depth := getDepth(path, root)
		if depth > config.IntVal(s.config.Scan.MaxDepth) {
			s.logger.Debug("skipping deep file", "path", path, "depth", depth, "max", config.IntVal(s.config.Scan.MaxDepth))
			return nil
		}

		// Filter by extension
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		ext = strings.ToLower(ext)
		if !isValidExtension(ext, s.config.Scan.Extensions) {
			s.logger.Debug("skipping unknown extension", "path", path, "ext", ext)
			return nil
		}

		// Filter by size
		size := fi.Size()
		minSize := int64(config.IntVal(s.config.Scan.MinFileSizeKB) * 1024)
		maxSize := int64(config.IntVal(s.config.Scan.MaxFileSizeMB) * 1024 * 1024)

		if size < minSize {
			result.Stats.SkippedBySize++
			s.logger.Debug("skipping small file", "path", path, "size", size, "min", minSize)
			return nil
		}
		if size > maxSize {
			result.Stats.SkippedBySize++
			s.logger.Debug("skipping large file", "path", path, "size", size, "max", maxSize)
			return nil
		}

		// Calculate SHA-256
		hash, err := fileHash(path)
		if err != nil {
			s.logger.Warn("cannot hash file", "path", path, "error", err)
			return nil
		}

		// Detect language
		lang := detectLanguage(path)

		// Detect file type
		fileType := manifest.FileType(ext)

		// Extract metadata: page count, text layer
		pageCount := getPageCount(path, fileType)
		hasText := hasTextLayer(path, fileType)

		// Build document
		doc := manifest.Document{
			ID:           hash,
			SourcePath:   path,
			FileType:     fileType,
			FileSize:     size,
			PageCount:    pageCount,
			HasTextLayer: hasText,
			NeedsOCR:     !hasText,
			Language:     &lang,
		}

		// Insert into manifest
		if err := s.manifest.InsertDocument(ctx, &doc); err != nil {
			s.logger.Warn("cannot insert document", "path", path, "error", err)
			result.Errors = append(result.Errors, ScanError{Path: path, Error: err})
			return nil
		}

		result.Documents = append(result.Documents, doc)
		result.Stats.TotalFiles++
		result.Stats.ByType[string(fileType)]++
		result.Stats.TotalSize += size
		if hasText {
			result.Stats.WithTextLayer++
		} else {
			result.Stats.NeedingOCR++
		}

		// Send progress event
		if progress != nil {
			progress <- &Event{
				Phase:   "scan",
				Current: len(result.Documents),
				Message: fmt.Sprintf("Found %s", filepath.Base(path)),
			}
		}

		return nil
	})
	if err != nil {
		// On context cancellation or deadline, return partial results
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, fmt.Errorf("scan interrupted: %w", err)
		}
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	s.logger.Info("scan complete",
		"total", result.Stats.TotalFiles,
		"size", result.Stats.TotalSize,
		"by_type", result.Stats.ByType,
		"errors", len(result.Errors),
	)

	return result, nil
}

// shouldExcludeDir checks if a directory should be skipped.
func (s *Scanner) shouldExcludeDir(path, root string) bool {
	// Always skip hidden directories
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." {
		return true
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	for _, excluded := range s.config.Scan.ExcludeDirs {
		if strings.Contains(rel, excluded) {
			return true
		}
	}

	return false
}

// getDepth returns the depth of a path relative to root.
func getDepth(path, root string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	if rel == "." {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1
}

// isValidExtension checks if ext is in the allowed list.
func isValidExtension(ext string, allowed []string) bool {
	for _, a := range allowed {
		if ext == strings.ToLower(a) {
			return true
		}
	}
	return false
}

// fileHash calculates SHA-256 of a file using streaming.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only, error not useful

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// getPageCount extracts the number of pages from a document.
// Uses pdfinfo for PDFs and djvused for DjVu files.
// Returns nil if the tool is unavailable or the file cannot be read.
func getPageCount(path string, fileType manifest.FileType) *int {
	switch fileType {
	case manifest.FileTypePDF:
		return pdfPageCount(path)
	case manifest.FileTypeDjVu:
		return djvuPageCount(path)
	}
	return nil
}

// pdfPageCount runs pdfinfo and parses the Pages field.
func pdfPageCount(path string) *int {
	out, err := exec.Command("pdfinfo", path).Output() //nolint:gosec // user-provided path
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if n, err := strconv.Atoi(fields[1]); err == nil {
					return &n
				}
			}
		}
	}
	return nil
}

// djvuPageCount runs djvused -e "n" and parses the integer output.
func djvuPageCount(path string) *int {
	out, err := exec.Command("djvused", "-e", "n", path).Output() //nolint:gosec // user-provided path
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	if n, err := strconv.Atoi(s); err == nil {
		return &n
	}
	return nil
}

// hasTextLayer checks whether a document has extractable text.
// Uses pdftotext for PDFs and djvutxt for DjVu files.
func hasTextLayer(path string, fileType manifest.FileType) bool {
	switch fileType {
	case manifest.FileTypePDF:
		return pdfHasTextLayer(path)
	case manifest.FileTypeDjVu:
		return djvuHasTextLayer(path)
	}
	// DOCX, EPUB, DOC: assume they have text
	return true
}

// pdfHasTextLayer extracts text from the first page and checks if it's non-empty.
func pdfHasTextLayer(path string) bool {
	out, err := exec.Command("pdftotext", "-l", "1", path, "-").Output() //nolint:gosec // user-provided path
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// djvuHasTextLayer extracts text and checks if it's non-empty.
func djvuHasTextLayer(path string) bool {
	out, err := exec.Command("djvutxt", path).Output() //nolint:gosec // user-provided path
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// detectLanguage does basic language detection from file path.
func detectLanguage(path string) manifest.Language {
	lower := strings.ToLower(path)

	// Spanish indicators
	spanishWords := []string{
		"acta", "notas", "tesis", "ejercicios", "fisica",
		"quimica", "biologia", "matematicas", "espanol",
		"clase", "parcial", "final", "universidad",
	}
	for _, w := range spanishWords {
		if strings.Contains(lower, w) {
			return manifest.LanguageES
		}
	}

	return manifest.LanguageEN
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}
