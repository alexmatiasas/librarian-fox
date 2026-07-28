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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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
	config    *config.Config
	manifest  *manifest.DB
	logger    *slog.Logger
	DryRun    bool // skip DB writes
	StatsOnly bool // Phase 1 only: count files, no hashing/subprocess/DB
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
	SkippedOnMtime  int
	WithTextLayer   int
	NeedingOCR      int
	Duration        time.Duration
	CollectMs       int64
	ProcessMs       int64
}

// ScanError records a file that could not be processed.
type ScanError struct {
	Path  string
	Error error
}

type fileEntry struct {
	path     string
	info     fs.FileInfo
	fileType manifest.FileType
	depth    int
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
	start := time.Now()
	result := &Result{
		Stats: Stats{ByType: make(map[string]int)},
	}

	root = expandTilde(root)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}
	root = absRoot

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("cannot access root directory %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path %s is not a directory", root)
	}

	s.logger.Info("starting scan", "root", root)
	s.checkTools()

	// Phase 1: collect candidate file entries (filtering only, no processing)
	collectStart := time.Now()
	candidates, err := s.collectCandidates(ctx, root, result, progress)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			result.Stats.Duration = time.Since(start)
			return result, fmt.Errorf("scan interrupted: %w", err)
		}
		return nil, fmt.Errorf("walk directory: %w", err)
	}
	collectTime := time.Since(collectStart)

	totalFiles := len(candidates)
	if totalFiles == 0 {
		result.Stats.Duration = time.Since(start)
		s.logger.Info("scan complete — no files found")
		return result, nil
	}

	if s.StatsOnly {
		// Phase 2 skip: count only, no hashing/subprocess/DB
		for _, c := range candidates {
			result.Stats.TotalFiles++
			result.Stats.ByType[string(c.fileType)]++
			result.Stats.TotalSize += c.info.Size()
		}
		result.Stats.Duration = time.Since(start)
		result.Stats.CollectMs = collectTime.Milliseconds()
		s.logger.Info("scan complete (stats only)",
			"total", result.Stats.TotalFiles,
			"size", result.Stats.TotalSize,
			"by_type", result.Stats.ByType,
			"collect_ms", collectTime.Milliseconds(),
			"total_ms", result.Stats.Duration.Milliseconds(),
		)
		return result, nil
	}

	s.logger.Info("processing files",
		"candidates", totalFiles,
		"collect_ms", collectTime.Milliseconds(),
	)

	// Phase 2: process candidates concurrently with a worker pool
	processStart := time.Now()
	s.processCandidates(ctx, candidates, totalFiles, result, progress)
	processTime := time.Since(processStart)

	result.Stats.Duration = time.Since(start)
	result.Stats.CollectMs = collectTime.Milliseconds()
	result.Stats.ProcessMs = processTime.Milliseconds()

	s.logger.Info("scan complete",
		"total", result.Stats.TotalFiles,
		"size", result.Stats.TotalSize,
		"by_type", result.Stats.ByType,
		"skipped_on_mtime", result.Stats.SkippedOnMtime,
		"errors", len(result.Errors),
		"collect_ms", collectTime.Milliseconds(),
		"process_ms", processTime.Milliseconds(),
		"total_ms", result.Stats.Duration.Milliseconds(),
		"files_per_sec", formatFPS(result.Stats.TotalFiles, result.Stats.Duration),
	)

	return result, nil
}

// collectCandidates walks the directory tree and collects eligible file entries.
// Sends periodic progress events with Phase "collect" if progress channel is non-nil.
func (s *Scanner) collectCandidates(ctx context.Context, root string, result *Result, progress chan<- *Event) ([]fileEntry, error) {
	var candidates []fileEntry
	found := 0

	progressTicker := time.NewTicker(500 * time.Millisecond)
	defer progressTicker.Stop()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.logger.Warn("cannot access path", "path", path, "error", err)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check symlink BEFORE stat to avoid unnecessary syscalls
		if d.Type()&fs.ModeSymlink != 0 {
			result.Stats.SymlinksSkipped++
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			s.logger.Warn("cannot get file info", "path", path, "error", err)
			return nil
		}

		if d.IsDir() {
			if s.shouldExcludeDir(path, root) {
				result.Stats.SkippedByDir++
				return filepath.SkipDir
			}
			return nil
		}

		depth := getDepth(path, root)
		if depth > config.IntVal(s.config.Scan.MaxDepth) {
			return nil
		}

		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		ext = strings.ToLower(ext)
		if !isValidExtension(ext, s.config.Scan.Extensions) {
			return nil
		}

		size := fi.Size()
		minSize := int64(config.IntVal(s.config.Scan.MinFileSizeKB) * 1024)
		maxSize := int64(config.IntVal(s.config.Scan.MaxFileSizeMB) * 1024 * 1024)

		if size < minSize || size > maxSize {
			result.Stats.SkippedBySize++
			return nil
		}

		candidates = append(candidates, fileEntry{
			path:     path,
			info:     fi,
			fileType: manifest.FileType(ext),
			depth:    depth,
		})
		found++

		// Periodic collect progress (~500ms)
		select {
		case <-progressTicker.C:
			if progress != nil && found > 0 {
				select {
				case progress <- &Event{Phase: "collect", Current: found}:
				default:
				}
			}
		default:
		}

		return nil
	})

	// Final collect event with total known
	if progress != nil && found > 0 {
		progress <- &Event{Phase: "collect", Current: found}
	}

	return candidates, err
}

// processCandidates processes file entries concurrently using a worker pool.
func (s *Scanner) processCandidates(ctx context.Context, candidates []fileEntry, total int, result *Result, progress chan<- *Event) {
	maxConc := maxConcurrentFiles(s.config.Scan.MaxConcurrentFiles)
	if maxConc > len(candidates) {
		maxConc = len(candidates)
	}
	if maxConc < 1 {
		maxConc = 1
	}

	sem := make(chan struct{}, maxConc)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, entry := range candidates {
		wg.Add(1)
		go func(entry fileEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			doc, skipped, err := s.processFile(ctx, entry)
			if err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, ScanError{Path: entry.path, Error: err})
				mu.Unlock()
				return
			}

			mu.Lock()
			if skipped {
				result.Stats.SkippedOnMtime++
			} else {
				result.Documents = append(result.Documents, *doc)
			}
			result.Stats.TotalFiles++
			result.Stats.ByType[string(doc.FileType)]++
			result.Stats.TotalSize += doc.FileSize
			if doc.HasTextLayer {
				result.Stats.WithTextLayer++
			} else {
				result.Stats.NeedingOCR++
			}
			current := result.Stats.TotalFiles
			mu.Unlock()

			if progress != nil {
				progress <- &Event{
					Phase:   "scan",
					Current: current,
					Total:   total,
					Message: fmt.Sprintf("Found %s", filepath.Base(entry.path)),
				}
			}
		}(entry)
	}

	wg.Wait()
}

// processFile handles a single file entry: skip check, hash, subprocess, DB insert.
// Returns the document, whether it was skipped (unchanged since last scan), and any error.
func (s *Scanner) processFile(ctx context.Context, entry fileEntry) (*manifest.Document, bool, error) {
	// Check if file can be skipped (unchanged mtime + size)
	mtime := entry.info.ModTime().UTC().Format(time.RFC3339)
	size := entry.info.Size()

	existing, err := s.manifest.FindByPath(ctx, entry.path)
	if err != nil {
		return nil, false, fmt.Errorf("checking existing document: %w", err)
	}

	if existing != nil && existing.FileSize == size && existing.ModifiedAt != nil && *existing.ModifiedAt == mtime {
		// File unchanged since last scan — build doc from existing record
		s.logger.Debug("skipping unchanged file", "path", entry.path)
		existing.SourcePath = entry.path
		return existing, true, nil
	}

	// File is new or changed — process fully
	hash, err := fileHash(entry.path)
	if err != nil {
		return nil, false, fmt.Errorf("hashing file: %w", err)
	}

	lang := detectLanguage(entry.path)
	pageCount := getPageCount(entry.path, entry.fileType)
	hasText := hasTextLayer(entry.path, entry.fileType)

	doc := &manifest.Document{
		ID:           hash,
		SourcePath:   entry.path,
		FileType:     entry.fileType,
		FileSize:     size,
		PageCount:    pageCount,
		HasTextLayer: hasText,
		NeedsOCR:     !hasText,
		Language:     &lang,
		ModifiedAt:   &mtime,
	}

	if !s.DryRun {
		if err := s.manifest.InsertDocument(ctx, doc); err != nil {
			return nil, false, fmt.Errorf("inserting document: %w", err)
		}
	}

	return doc, false, nil
}

// shouldExcludeDir checks if a directory should be skipped.
func (s *Scanner) shouldExcludeDir(path, root string) bool {
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

func isValidExtension(ext string, allowed []string) bool {
	for _, a := range allowed {
		if ext == strings.ToLower(a) {
			return true
		}
	}
	return false
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func getPageCount(path string, fileType manifest.FileType) *int {
	switch fileType {
	case manifest.FileTypePDF:
		return pdfPageCount(path)
	case manifest.FileTypeDjVu:
		return djvuPageCount(path)
	}
	return nil
}

func pdfPageCount(path string) *int {
	out, err := exec.Command("pdfinfo", path).Output()
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

func djvuPageCount(path string) *int {
	out, err := exec.Command("djvused", "-e", "n", path).Output()
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	if n, err := strconv.Atoi(s); err == nil {
		return &n
	}
	return nil
}

func hasTextLayer(path string, fileType manifest.FileType) bool {
	switch fileType {
	case manifest.FileTypePDF:
		return pdfHasTextLayer(path)
	case manifest.FileTypeDjVu:
		return djvuHasTextLayer(path)
	}
	return true
}

func pdfHasTextLayer(path string) bool {
	out, err := exec.Command("pdftotext", "-l", "1", path, "-").Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func djvuHasTextLayer(path string) bool {
	out, err := exec.Command("djvutxt", path).Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func detectLanguage(path string) manifest.Language {
	lower := strings.ToLower(path)

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

// maxConcurrentFiles returns the number of goroutines to use.
// If explicitly set in config, use that. Otherwise auto-detect: NumCPU * 2, capped at 8.
func maxConcurrentFiles(cfgVal *int) int {
	if cfgVal != nil {
		return *cfgVal
	}
	n := runtime.NumCPU() * 2
	if n > 8 {
		n = 8
	}
	return n
}

// checkTools verifies that external binaries are available and logs warnings for missing ones.
func (s *Scanner) checkTools() {
	for _, tool := range []string{"pdfinfo", "pdftotext", "djvused", "djvutxt"} {
		if _, err := exec.LookPath(tool); err != nil {
			s.logger.Warn("tool not found — some features disabled",
				"tool", tool,
			)
		}
	}
}

func formatFPS(count int, dur time.Duration) string {
	if dur <= 0 {
		return "0"
	}
	fps := float64(count) / dur.Seconds()
	return fmt.Sprintf("%.1f", fps)
}
