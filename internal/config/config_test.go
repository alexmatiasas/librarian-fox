package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/jsonschema"
)

// ============================================================================
// Helper tests
// ============================================================================

func TestHelpers(t *testing.T) {
	// strPtr/strVal
	s := strPtr("hello")
	if strVal(s) != "hello" {
		t.Errorf("strVal(strPtr('hello')) = %q", strVal(s))
	}
	if strVal(nil) != "" {
		t.Errorf("strVal(nil) = %q, want empty", strVal(nil))
	}

	// boolPtr/boolVal
	b := boolPtr(true)
	if !boolVal(b) {
		t.Error("boolVal(boolPtr(true)) should be true")
	}
	if boolVal(nil) {
		t.Error("boolVal(nil) should be false")
	}

	// intPtr/intVal
	i := intPtr(42)
	if intVal(i) != 42 {
		t.Errorf("intVal(intPtr(42)) = %d", intVal(i))
	}
	if intVal(nil) != 0 {
		t.Errorf("intVal(nil) = %d, want 0", intVal(nil))
	}

	// floatPtr/floatVal
	f := floatPtr(3.14)
	if floatVal(f) != 3.14 {
		t.Errorf("floatVal(floatPtr(3.14)) = %f", floatVal(f))
	}
	if floatVal(nil) != 0 {
		t.Errorf("floatVal(nil) = %f, want 0", floatVal(nil))
	}
}

// ============================================================================
// DefaultConfig tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// General
	if cfg.General == nil {
		t.Fatal("General config is nil")
	}
	if strVal(cfg.General.Root) != "~" {
		t.Errorf("General.Root = %q, want ~", strVal(cfg.General.Root))
	}
	if strVal(cfg.General.LibraryDir) != "~/Library" {
		t.Errorf("General.LibraryDir = %q, want ~/Library", strVal(cfg.General.LibraryDir))
	}
	if strVal(cfg.General.ManifestPath) != "~/.local/share/lfx/manifest.db" {
		t.Errorf("General.ManifestPath = %q", strVal(cfg.General.ManifestPath))
	}
	if strVal(cfg.General.LogLevel) != "info" {
		t.Errorf("General.LogLevel = %q, want info", strVal(cfg.General.LogLevel))
	}
	if cfg.General.DryRun == nil || !*cfg.General.DryRun {
		t.Error("General.DryRun should default to true")
	}

	// Scan
	if cfg.Scan == nil {
		t.Fatal("Scan config is nil")
	}
	if intVal(cfg.Scan.MaxDepth) != 5 {
		t.Errorf("Scan.MaxDepth = %d, want 5", intVal(cfg.Scan.MaxDepth))
	}
	if intVal(cfg.Scan.MinFileSizeKB) != 10 {
		t.Errorf("Scan.MinFileSizeKB = %d, want 10", intVal(cfg.Scan.MinFileSizeKB))
	}
	if intVal(cfg.Scan.MaxFileSizeMB) != 500 {
		t.Errorf("Scan.MaxFileSizeMB = %d, want 500", intVal(cfg.Scan.MaxFileSizeMB))
	}
	if len(cfg.Scan.ExcludeDirs) == 0 {
		t.Error("Scan.ExcludeDirs should not be empty")
	}
	if len(cfg.Scan.Extensions) != 4 {
		t.Errorf("Scan.Extensions = %v, want 4 extensions", cfg.Scan.Extensions)
	}

	// LLM
	if cfg.LLM == nil {
		t.Fatal("LLM config is nil")
	}
	if strVal(cfg.LLM.Provider) != "ollama" {
		t.Errorf("LLM.Provider = %q, want ollama", strVal(cfg.LLM.Provider))
	}
	if strVal(cfg.LLM.Model) != "qwen3:8b" {
		t.Errorf("LLM.Model = %q, want qwen3:8b", strVal(cfg.LLM.Model))
	}
	if strVal(cfg.LLM.FallbackModel) != "llama3.3:8b" {
		t.Errorf("LLM.FallbackModel = %q, want llama3.3:8b", strVal(cfg.LLM.FallbackModel))
	}
	if floatVal(cfg.LLM.ConfidenceThreshold) != 0.7 {
		t.Errorf("LLM.ConfidenceThreshold = %f, want 0.7", floatVal(cfg.LLM.ConfidenceThreshold))
	}
	if floatVal(cfg.LLM.ConfidenceWarning) != 0.9 {
		t.Errorf("LLM.ConfidenceWarning = %f, want 0.9", floatVal(cfg.LLM.ConfidenceWarning))
	}
	if floatVal(cfg.LLM.Temperature) != 0.2 {
		t.Errorf("LLM.Temperature = %f, want 0.2", floatVal(cfg.LLM.Temperature))
	}
	if intVal(cfg.LLM.MaxConcurrent) != 3 {
		t.Errorf("LLM.MaxConcurrent = %d, want 3", intVal(cfg.LLM.MaxConcurrent))
	}
	if intVal(cfg.LLM.TimeoutSeconds) != 60 {
		t.Errorf("LLM.TimeoutSeconds = %d, want 60", intVal(cfg.LLM.TimeoutSeconds))
	}
	if cfg.LLM.Retry == nil || !*cfg.LLM.Retry {
		t.Error("LLM.Retry should default to true")
	}
	if intVal(cfg.LLM.MaxRetries) != 2 {
		t.Errorf("LLM.MaxRetries = %d, want 2", intVal(cfg.LLM.MaxRetries))
	}

	// Library
	if cfg.Library == nil {
		t.Fatal("Library config is nil")
	}
	if strVal(cfg.Library.Layout) != "{tipo}/{subject}" {
		t.Errorf("Library.Layout = %q, want {tipo}/{subject}", strVal(cfg.Library.Layout))
	}
	if cfg.Library.Types == nil || len(cfg.Library.Types.ValidTypes) != 8 {
		t.Errorf("Library.Types.ValidTypes should have 8 types")
	}
	if cfg.Library.Subjects == nil || strVal(cfg.Library.Subjects.Strategy) != "emergente" {
		t.Errorf("Library.Subjects.Strategy = %q, want emergente", strVal(cfg.Library.Subjects.Strategy))
	}

	// Metadata
	if cfg.Metadata == nil {
		t.Fatal("Metadata config is nil")
	}
	if strVal(cfg.Metadata.EbookMetaPath) != "ebook-meta" {
		t.Errorf("Metadata.EbookMetaPath = %q, want ebook-meta", strVal(cfg.Metadata.EbookMetaPath))
	}

	// OCR
	if cfg.OCR == nil {
		t.Fatal("OCR config is nil")
	}
	if intVal(cfg.OCR.MaxConcurrent) != 2 {
		t.Errorf("OCR.MaxConcurrent = %d, want 2", intVal(cfg.OCR.MaxConcurrent))
	}
	if cfg.OCR.Quality == nil {
		t.Fatal("OCR.Quality is nil")
	}
	if floatVal(cfg.OCR.Quality.BlurThreshold) != 100.0 {
		t.Errorf("OCR.Quality.BlurThreshold = %f, want 100.0", floatVal(cfg.OCR.Quality.BlurThreshold))
	}

	// Apply
	if cfg.Apply == nil {
		t.Fatal("Apply config is nil")
	}
	if strVal(cfg.Apply.FilenameTemplate) != "{autor} - {titulo} ({anio})" {
		t.Errorf("Apply.FilenameTemplate = %q", strVal(cfg.Apply.FilenameTemplate))
	}
	if strVal(cfg.Apply.Compression) != "ocrmypdf" {
		t.Errorf("Apply.Compression = %q, want ocrmypdf", strVal(cfg.Apply.Compression))
	}
	if strVal(cfg.Apply.Conflict) != "suffix" {
		t.Errorf("Apply.Conflict = %q, want suffix", strVal(cfg.Apply.Conflict))
	}
	if cfg.Apply.BackupCompress == nil || !*cfg.Apply.BackupCompress {
		t.Error("Apply.BackupCompress should default to true")
	}
	if intVal(cfg.Apply.BackupMaxCount) != 5 {
		t.Errorf("Apply.BackupMaxCount = %d, want 5", intVal(cfg.Apply.BackupMaxCount))
	}

	// Kavita
	if cfg.Kavita == nil {
		t.Fatal("Kavita config is nil")
	}
	if cfg.Kavita.Enabled != nil && *cfg.Kavita.Enabled {
		t.Error("Kavita.Enabled should default to false")
	}
	if intVal(cfg.Kavita.Port) != 5000 {
		t.Errorf("Kavita.Port = %d, want 5000", intVal(cfg.Kavita.Port))
	}
}

// ============================================================================
// expandTilde tests
// ============================================================================

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~", home},
		{"~/Documents", filepath.Join(home, "Documents")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		got := expandTilde(tt.input)
		if got != tt.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================================
// Merge tests
// ============================================================================

func TestMergeGeneral(t *testing.T) {
	dst := &GeneralConfig{
		Root:         strPtr("~"),
		LibraryDir:   strPtr("~/Library"),
		ManifestPath: strPtr("~/.local/share/lfx/manifest.db"),
		LogLevel:     strPtr("info"),
		DryRun:       boolPtr(true),
	}
	src := &GeneralConfig{
		Root:     strPtr("/custom/root"),
		LogLevel: strPtr("debug"),
	}

	mergeGeneral(dst, src)

	if strVal(dst.Root) != "/custom/root" {
		t.Errorf("Root = %q, want /custom/root", strVal(dst.Root))
	}
	if strVal(dst.LibraryDir) != "~/Library" {
		t.Errorf("LibraryDir should not change, got %q", strVal(dst.LibraryDir))
	}
	if strVal(dst.LogLevel) != "debug" {
		t.Errorf("LogLevel = %q, want debug", strVal(dst.LogLevel))
	}
	if dst.DryRun == nil || !*dst.DryRun {
		t.Error("DryRun should remain true")
	}
}

func TestMergeGeneral_DryRunOverride(t *testing.T) {
	dst := &GeneralConfig{
		Root:   strPtr("~"),
		DryRun: boolPtr(true),
	}
	src := &GeneralConfig{
		DryRun: boolPtr(false),
	}

	mergeGeneral(dst, src)

	if dst.DryRun == nil {
		t.Fatal("DryRun should not be nil")
	}
	if *dst.DryRun {
		t.Error("DryRun should be overridden to false")
	}
}

func TestMergeGeneral_DryRunNilSrc(t *testing.T) {
	dst := &GeneralConfig{
		Root:   strPtr("~"),
		DryRun: boolPtr(true),
	}
	src := &GeneralConfig{
		Root: strPtr("/new/root"),
	}

	mergeGeneral(dst, src)

	if dst.DryRun == nil || !*dst.DryRun {
		t.Error("DryRun should remain true when src.DryRun is nil")
	}
}

func TestMergeScan(t *testing.T) {
	dst := &ScanConfig{
		MaxDepth:      intPtr(5),
		MinFileSizeKB: intPtr(10),
		MaxFileSizeMB: intPtr(500),
		Extensions:    []string{"pdf"},
		ExcludeDirs:   []string{"Library"},
	}
	src := &ScanConfig{
		MaxDepth:      intPtr(3),
		MinFileSizeKB: intPtr(20),
		MaxFileSizeMB: intPtr(100),
		Extensions:    []string{"pdf", "djvu"},
		ExcludeDirs:   []string{"node_modules"},
	}

	mergeScan(dst, src)

	if intVal(dst.MaxDepth) != 3 {
		t.Errorf("MaxDepth = %d, want 3", intVal(dst.MaxDepth))
	}
	if intVal(dst.MinFileSizeKB) != 20 {
		t.Errorf("MinFileSizeKB = %d, want 20", intVal(dst.MinFileSizeKB))
	}
	if intVal(dst.MaxFileSizeMB) != 100 {
		t.Errorf("MaxFileSizeMB = %d, want 100", intVal(dst.MaxFileSizeMB))
	}
	if len(dst.Extensions) != 2 {
		t.Errorf("Extensions = %v, want [pdf djvu]", dst.Extensions)
	}
	if len(dst.ExcludeDirs) != 1 || dst.ExcludeDirs[0] != "node_modules" {
		t.Errorf("ExcludeDirs = %v, want [node_modules]", dst.ExcludeDirs)
	}
}

func TestMergeLLM(t *testing.T) {
	dst := &LLMConfig{
		Provider:            strPtr("ollama"),
		Model:               strPtr("qwen3:8b"),
		FallbackModel:       strPtr("llama3.3:8b"),
		ConfidenceThreshold: floatPtr(0.7),
		ConfidenceWarning:   floatPtr(0.9),
		Temperature:         floatPtr(0.2),
		MaxConcurrent:       intPtr(3),
		TimeoutSeconds:      intPtr(60),
		LocalOnlyCategories: []string{"Personal"},
		Retry:               boolPtr(true),
		MaxRetries:          intPtr(2),
	}
	src := &LLMConfig{
		Provider:            strPtr("groq"),
		Model:               strPtr("llama3.1:8b"),
		FallbackModel:       strPtr("qwen3:8b"),
		ConfidenceThreshold: floatPtr(0.8),
		ConfidenceWarning:   floatPtr(0.95),
		Temperature:         floatPtr(0.5),
		MaxConcurrent:       intPtr(5),
		TimeoutSeconds:      intPtr(120),
		LocalOnlyCategories: []string{"Admin"},
		Retry:               boolPtr(false),
		MaxRetries:          intPtr(3),
	}

	mergeLLM(dst, src)

	if strVal(dst.Provider) != "groq" {
		t.Errorf("Provider = %q, want groq", strVal(dst.Provider))
	}
	if strVal(dst.Model) != "llama3.1:8b" {
		t.Errorf("Model = %q, want llama3.1:8b", strVal(dst.Model))
	}
	if strVal(dst.FallbackModel) != "qwen3:8b" {
		t.Errorf("FallbackModel = %q, want qwen3:8b", strVal(dst.FallbackModel))
	}
	if floatVal(dst.ConfidenceThreshold) != 0.8 {
		t.Errorf("ConfidenceThreshold = %f, want 0.8", floatVal(dst.ConfidenceThreshold))
	}
	if floatVal(dst.ConfidenceWarning) != 0.95 {
		t.Errorf("ConfidenceWarning = %f, want 0.95", floatVal(dst.ConfidenceWarning))
	}
	if floatVal(dst.Temperature) != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", floatVal(dst.Temperature))
	}
	if intVal(dst.MaxConcurrent) != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", intVal(dst.MaxConcurrent))
	}
	if intVal(dst.TimeoutSeconds) != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", intVal(dst.TimeoutSeconds))
	}
	if dst.LocalOnlyCategories[0] != "Admin" {
		t.Errorf("LocalOnlyCategories = %v, want [Admin]", dst.LocalOnlyCategories)
	}
	if dst.Retry == nil || *dst.Retry {
		t.Error("Retry should be false")
	}
	if intVal(dst.MaxRetries) != 3 {
		t.Errorf("MaxRetries = %d, want 3", intVal(dst.MaxRetries))
	}
}

func TestMergeLibrary(t *testing.T) {
	dst := &LibraryConfig{
		Layout: strPtr("{tipo}/{subject}"),
		Types: &LibraryTypes{
			ValidTypes: []string{"Libro", "Paper"},
		},
		Subjects: &LibrarySubjects{
			Strategy:       strPtr("emergente"),
			MergeThreshold: floatPtr(0.8),
		},
	}
	src := &LibraryConfig{
		Layout: strPtr("{subject}/{tipo}"),
		Subjects: &LibrarySubjects{
			MergeThreshold: floatPtr(0.9),
		},
	}

	mergeLibrary(dst, src)

	if strVal(dst.Layout) != "{subject}/{tipo}" {
		t.Errorf("Layout = %q, want {subject}/{tipo}", strVal(dst.Layout))
	}
	if len(dst.Types.ValidTypes) != 2 {
		t.Errorf("Types.ValidTypes should not change, got %v", dst.Types.ValidTypes)
	}
	if strVal(dst.Subjects.Strategy) != "emergente" {
		t.Errorf("Strategy should not change, got %q", strVal(dst.Subjects.Strategy))
	}
	if floatVal(dst.Subjects.MergeThreshold) != 0.9 {
		t.Errorf("MergeThreshold = %f, want 0.9", floatVal(dst.Subjects.MergeThreshold))
	}
}

func TestMergeLibrary_StrategyOverride(t *testing.T) {
	dst := &LibraryConfig{
		Subjects: &LibrarySubjects{
			Strategy: strPtr("emergente"),
		},
	}
	src := &LibraryConfig{
		Subjects: &LibrarySubjects{
			Strategy: strPtr("predefinida"),
		},
	}

	mergeLibrary(dst, src)

	if strVal(dst.Subjects.Strategy) != "predefinida" {
		t.Errorf("Strategy = %q, want predefinida", strVal(dst.Subjects.Strategy))
	}
}

func TestMergeLibrary_NilSections(t *testing.T) {
	dst := &LibraryConfig{
		Layout: strPtr("{tipo}/{subject}"),
	}
	src := &LibraryConfig{
		Types: &LibraryTypes{
			ValidTypes: []string{"Libro"},
		},
	}

	mergeLibrary(dst, src)

	if dst.Types == nil {
		t.Fatal("Types should be created from src")
	}
	if len(dst.Types.ValidTypes) != 1 || dst.Types.ValidTypes[0] != "Libro" {
		t.Errorf("Types.ValidTypes = %v, want [Libro]", dst.Types.ValidTypes)
	}
}

func TestMergeMetadata(t *testing.T) {
	dst := &MetadataConfig{
		EbookMetaPath:     strPtr("ebook-meta"),
		FetchMetadataPath: strPtr("fetch-ebook-metadata"),
		MetadataSources:   []string{"google_books"},
	}
	src := &MetadataConfig{
		MetadataSources: []string{"open_library", "amazon"},
	}

	mergeMetadata(dst, src)

	if strVal(dst.EbookMetaPath) != "ebook-meta" {
		t.Errorf("EbookMetaPath should not change, got %q", strVal(dst.EbookMetaPath))
	}
	if strVal(dst.FetchMetadataPath) != "fetch-ebook-metadata" {
		t.Errorf("FetchMetadataPath should not change, got %q", strVal(dst.FetchMetadataPath))
	}
	if len(dst.MetadataSources) != 2 {
		t.Errorf("MetadataSources should be overwritten, got %v", dst.MetadataSources)
	}
}

func TestMergeMetadata_PathOverrides(t *testing.T) {
	dst := &MetadataConfig{
		EbookMetaPath:     strPtr("ebook-meta"),
		FetchMetadataPath: strPtr("fetch-ebook-metadata"),
	}
	src := &MetadataConfig{
		EbookMetaPath:     strPtr("/usr/local/bin/ebook-meta"),
		FetchMetadataPath: strPtr("/usr/local/bin/fetch-ebook-metadata"),
	}

	mergeMetadata(dst, src)

	if strVal(dst.EbookMetaPath) != "/usr/local/bin/ebook-meta" {
		t.Errorf("EbookMetaPath = %q, want /usr/local/bin/ebook-meta", strVal(dst.EbookMetaPath))
	}
	if strVal(dst.FetchMetadataPath) != "/usr/local/bin/fetch-ebook-metadata" {
		t.Errorf("FetchMetadataPath = %q, want /usr/local/bin/fetch-ebook-metadata", strVal(dst.FetchMetadataPath))
	}
}

func TestMergeOCR(t *testing.T) {
	dst := &OCRConfig{
		Languages:     []string{"eng"},
		MaxConcurrent: intPtr(2),
		Quality: &OCRQuality{
			BlurThreshold:     floatPtr(100.0),
			SkewThreshold:     floatPtr(2.0),
			ContrastMinSpread: floatPtr(30.0),
		},
	}
	src := &OCRConfig{
		Languages: []string{"spa", "deu"},
		Quality: &OCRQuality{
			BlurThreshold: floatPtr(150.0),
		},
	}

	mergeOCR(dst, src)

	if len(dst.Languages) != 2 || dst.Languages[0] != "spa" {
		t.Errorf("Languages = %v, want [spa deu]", dst.Languages)
	}
	if intVal(dst.MaxConcurrent) != 2 {
		t.Errorf("MaxConcurrent should not change, got %d", intVal(dst.MaxConcurrent))
	}
	if floatVal(dst.Quality.BlurThreshold) != 150.0 {
		t.Errorf("BlurThreshold = %f, want 150.0", floatVal(dst.Quality.BlurThreshold))
	}
	if floatVal(dst.Quality.SkewThreshold) != 2.0 {
		t.Errorf("SkewThreshold should not change, got %f", floatVal(dst.Quality.SkewThreshold))
	}
}

func TestMergeOCR_MaxConcurrentOverride(t *testing.T) {
	dst := &OCRConfig{
		MaxConcurrent: intPtr(2),
	}
	src := &OCRConfig{
		MaxConcurrent: intPtr(4),
	}

	mergeOCR(dst, src)

	if intVal(dst.MaxConcurrent) != 4 {
		t.Errorf("MaxConcurrent = %d, want 4", intVal(dst.MaxConcurrent))
	}
}

func TestMergeOCR_NilQuality(t *testing.T) {
	dst := &OCRConfig{
		Languages: []string{"eng"},
	}
	src := &OCRConfig{
		Quality: &OCRQuality{
			BlurThreshold: floatPtr(150.0),
		},
	}

	mergeOCR(dst, src)

	if dst.Quality == nil {
		t.Fatal("Quality should be created from src")
	}
	if floatVal(dst.Quality.BlurThreshold) != 150.0 {
		t.Errorf("BlurThreshold = %f, want 150.0", floatVal(dst.Quality.BlurThreshold))
	}
}

func TestMergeApply(t *testing.T) {
	dst := &ApplyConfig{
		FilenameTemplate: strPtr("{autor} - {titulo} ({anio})"),
		Compression:      strPtr("ocrmypdf"),
		Conflict:         strPtr("suffix"),
		BackupDir:        strPtr("{library_dir}/.backups"),
		BackupCompress:   boolPtr(true),
		BackupMaxCount:   intPtr(5),
		BackupMaxAgeDays: intPtr(30),
	}
	src := &ApplyConfig{
		FilenameTemplate: strPtr("{titulo} - {autor}.pdf"),
		Compression:      strPtr("ghostscript"),
		Conflict:         strPtr("overwrite"),
		BackupDir:        strPtr("/backups"),
		BackupCompress:   boolPtr(false),
		BackupMaxCount:   intPtr(10),
		BackupMaxAgeDays: intPtr(60),
	}

	mergeApply(dst, src)

	if strVal(dst.FilenameTemplate) != "{titulo} - {autor}.pdf" {
		t.Errorf("FilenameTemplate = %q", strVal(dst.FilenameTemplate))
	}
	if strVal(dst.Compression) != "ghostscript" {
		t.Errorf("Compression = %q, want ghostscript", strVal(dst.Compression))
	}
	if strVal(dst.Conflict) != "overwrite" {
		t.Errorf("Conflict = %q, want overwrite", strVal(dst.Conflict))
	}
	if strVal(dst.BackupDir) != "/backups" {
		t.Errorf("BackupDir = %q, want /backups", strVal(dst.BackupDir))
	}
	if dst.BackupCompress == nil || *dst.BackupCompress {
		t.Error("BackupCompress should be false")
	}
	if intVal(dst.BackupMaxCount) != 10 {
		t.Errorf("BackupMaxCount = %d, want 10", intVal(dst.BackupMaxCount))
	}
	if intVal(dst.BackupMaxAgeDays) != 60 {
		t.Errorf("BackupMaxAgeDays = %d, want 60", intVal(dst.BackupMaxAgeDays))
	}
}

func TestMergeKavita(t *testing.T) {
	dst := &KavitaConfig{
		Enabled:           boolPtr(false),
		DockerComposePath: strPtr("~/.config/lfx/docker-compose.kavita.yml"),
		Port:              intPtr(5000),
	}
	src := &KavitaConfig{
		Enabled: boolPtr(true),
		Port:    intPtr(8080),
	}

	mergeKavita(dst, src)

	if dst.Enabled == nil || !*dst.Enabled {
		t.Error("Enabled should be true")
	}
	if intVal(dst.Port) != 8080 {
		t.Errorf("Port = %d, want 8080", intVal(dst.Port))
	}
	if strVal(dst.DockerComposePath) != "~/.config/lfx/docker-compose.kavita.yml" {
		t.Errorf("DockerComposePath should not change, got %q", strVal(dst.DockerComposePath))
	}
}

func TestMergeFullConfig(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		General: &GeneralConfig{
			Root:     strPtr("/custom"),
			LogLevel: strPtr("debug"),
		},
		LLM: &LLMConfig{
			Provider: strPtr("groq"),
		},
	}

	merge(dst, src)

	if strVal(dst.General.Root) != "/custom" {
		t.Errorf("General.Root = %q, want /custom", strVal(dst.General.Root))
	}
	if strVal(dst.LLM.Provider) != "groq" {
		t.Errorf("LLM.Provider = %q, want groq", strVal(dst.LLM.Provider))
	}
	// Preserved defaults
	if strVal(dst.LLM.Model) != "qwen3:8b" {
		t.Errorf("LLM.Model should be preserved, got %q", strVal(dst.LLM.Model))
	}
}

func TestMergeAllSections(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		Library: &LibraryConfig{
			Layout: strPtr("{subject}/{tipo}"),
		},
		Metadata: &MetadataConfig{
			MetadataSources: []string{"amazon"},
		},
		OCR: &OCRConfig{
			Languages: []string{"deu"},
		},
		Apply: &ApplyConfig{
			Compression: strPtr("ghostscript"),
		},
		Kavita: &KavitaConfig{
			Enabled: boolPtr(true),
			Port:    intPtr(8080),
		},
	}

	merge(dst, src)

	if strVal(dst.Library.Layout) != "{subject}/{tipo}" {
		t.Errorf("Library.Layout = %q", strVal(dst.Library.Layout))
	}
	if dst.Metadata.MetadataSources[0] != "amazon" {
		t.Errorf("Metadata.MetadataSources = %v", dst.Metadata.MetadataSources)
	}
	if dst.OCR.Languages[0] != "deu" {
		t.Errorf("OCR.Languages = %v", dst.OCR.Languages)
	}
	if strVal(dst.Apply.Compression) != "ghostscript" {
		t.Errorf("Apply.Compression = %q", strVal(dst.Apply.Compression))
	}
	if dst.Kavita.Enabled == nil || !*dst.Kavita.Enabled {
		t.Error("Kavita.Enabled should be true")
	}
}

// ============================================================================
// Load/LoadFrom tests
// ============================================================================

func TestLoad_Function(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
	if cfg.General == nil {
		t.Fatal("General config is nil")
	}
}

func TestLoadFrom(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.toml")

	content := `
[general]
root = "/tmp/test-root"
log_level = "debug"

[llm]
provider = "groq"
model = "llama3.3:8b"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if strVal(cfg.General.Root) != "/tmp/test-root" {
		t.Errorf("General.Root = %q, want /tmp/test-root", strVal(cfg.General.Root))
	}
	if strVal(cfg.General.LogLevel) != "debug" {
		t.Errorf("General.LogLevel = %q, want debug", strVal(cfg.General.LogLevel))
	}
	if strVal(cfg.LLM.Provider) != "groq" {
		t.Errorf("LLM.Provider = %q, want groq", strVal(cfg.LLM.Provider))
	}
	// Defaults should be preserved
	if floatVal(cfg.LLM.ConfidenceThreshold) != 0.7 {
		t.Errorf("LLM.ConfidenceThreshold = %f, want 0.7 (default preserved)", floatVal(cfg.LLM.ConfidenceThreshold))
	}
	// DryRun should remain true (not in config file)
	if cfg.General.DryRun == nil || !*cfg.General.DryRun {
		t.Error("General.DryRun should remain true when not set in config file")
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path.toml")
	if err == nil {
		t.Error("LoadFrom() with nonexistent file should return error")
	}
}

func TestLoadExplicitDryRunFalse(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.toml")

	content := `
[general]
dry_run = false
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if cfg.General.DryRun == nil {
		t.Fatal("DryRun should not be nil")
	}
	if *cfg.General.DryRun {
		t.Error("DryRun should be false when explicitly set to false in config file")
	}
}

func TestLoadFrom_MalformedTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.toml")

	content := `
[general]
root = 
log_level = invalid
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(configPath)
	if err == nil {
		t.Error("LoadFrom() with malformed TOML should return error")
	}
}

// ============================================================================
// Validate tests
// ============================================================================

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid defaults",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "missing general",
			cfg: &Config{
				LLM: &LLMConfig{Provider: strPtr("ollama")},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: &Config{
				General: &GeneralConfig{
					Root:         strPtr("~"),
					LibraryDir:   strPtr("~/Library"),
					ManifestPath: strPtr("~/.local/share/lfx/manifest.db"),
					LogLevel:     strPtr("invalid"),
					DryRun:       boolPtr(true),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid provider",
			cfg: &Config{
				General: &GeneralConfig{
					Root:         strPtr("~"),
					LibraryDir:   strPtr("~/Library"),
					ManifestPath: strPtr("~/.local/share/lfx/manifest.db"),
					LogLevel:     strPtr("info"),
					DryRun:       boolPtr(true),
				},
				LLM: &LLMConfig{
					Provider: strPtr("openai"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid compression",
			cfg: &Config{
				General: &GeneralConfig{
					Root:         strPtr("~"),
					LibraryDir:   strPtr("~/Library"),
					ManifestPath: strPtr("~/.local/share/lfx/manifest.db"),
					LogLevel:     strPtr("info"),
					DryRun:       boolPtr(true),
				},
				Apply: &ApplyConfig{
					Compression: strPtr("invalid"),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_EmptyRequiredFields(t *testing.T) {
	cfg := &Config{
		General: &GeneralConfig{
			Root:         strPtr(""),
			LibraryDir:   strPtr(""),
			ManifestPath: strPtr(""),
			LogLevel:     strPtr("info"),
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should fail with empty required fields")
	}
}

func TestValidate_OutOfRangeValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.ConfidenceThreshold = floatPtr(1.5)
	cfg.LLM.Temperature = floatPtr(3.0)

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should fail with out-of-range values")
	}
}

func TestValidate_InvalidConflict(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Apply.Conflict = strPtr("invalid")

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should fail with invalid conflict strategy")
	}
}

// ============================================================================
// JSON Schema Tests
// ============================================================================

func TestSchema_IsValidJSON(t *testing.T) {
	schema := jsonschema.Reflect(&Config{})

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal schema to JSON: %v", err)
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Generated schema is not valid JSON: %v", err)
	}
}

func TestSchema_HasRequiredStructure(t *testing.T) {
	schema := jsonschema.Reflect(&Config{})

	if len(schema.Definitions) == 0 {
		t.Error("Schema missing $defs")
	}
}

func TestSchema_HasAllSections(t *testing.T) {
	schema := jsonschema.Reflect(&Config{})

	expectedDefs := []string{
		"Config", "GeneralConfig", "ScanConfig", "LLMConfig",
		"LibraryConfig", "LibraryTypes", "LibrarySubjects",
		"MetadataConfig", "OCRConfig", "OCRQuality",
		"ApplyConfig", "KavitaConfig",
	}

	for _, name := range expectedDefs {
		if _, ok := schema.Definitions[name]; !ok {
			t.Errorf("Schema missing definition: %s", name)
		}
	}
}

func TestSchema_GeneralConfig_HasCorrectEnums(t *testing.T) {
	schema := jsonschema.Reflect(&Config{})

	general, ok := schema.Definitions["GeneralConfig"]
	if !ok {
		t.Fatal("GeneralConfig not found in schema")
	}

	logLevelProp, ok := general.Properties.Get("log_level")
	if !ok {
		t.Fatal("log_level property not found")
	}

	if len(logLevelProp.Enum) != 4 {
		t.Errorf("log_level should have 4 enum values, got %d", len(logLevelProp.Enum))
	}

	expectedEnums := []string{"debug", "info", "warn", "error"}
	for i, expected := range expectedEnums {
		if logLevelProp.Enum[i] != expected {
			t.Errorf("log_level enum[%d] = %q, want %q", i, logLevelProp.Enum[i], expected)
		}
	}
}

func TestSchema_LLMConfig_HasCorrectEnums(t *testing.T) {
	schema := jsonschema.Reflect(&Config{})

	llm, ok := schema.Definitions["LLMConfig"]
	if !ok {
		t.Fatal("LLMConfig not found in schema")
	}

	providerProp, ok := llm.Properties.Get("provider")
	if !ok {
		t.Fatal("provider property not found")
	}

	expectedProviders := []string{"ollama", "groq", "cerebras"}
	if len(providerProp.Enum) != len(expectedProviders) {
		t.Errorf("provider should have %d enum values, got %d", len(expectedProviders), len(providerProp.Enum))
	}
}

func TestSchema_ApplyConfig_HasCorrectEnums(t *testing.T) {
	schema := jsonschema.Reflect(&Config{})

	apply, ok := schema.Definitions["ApplyConfig"]
	if !ok {
		t.Fatal("ApplyConfig not found in schema")
	}

	compressionProp, ok := apply.Properties.Get("compression")
	if !ok {
		t.Fatal("compression property not found")
	}

	expectedCompression := []string{"ocrmypdf", "ghostscript", "none"}
	if len(compressionProp.Enum) != len(expectedCompression) {
		t.Errorf("compression should have %d enum values, got %d", len(expectedCompression), len(compressionProp.Enum))
	}

	conflictProp, ok := apply.Properties.Get("conflict")
	if !ok {
		t.Fatal("conflict property not found")
	}

	expectedConflict := []string{"suffix", "overwrite", "skip", "ask"}
	if len(conflictProp.Enum) != len(expectedConflict) {
		t.Errorf("conflict should have %d enum values, got %d", len(expectedConflict), len(conflictProp.Enum))
	}
}

func TestSchema_Defaults_MatchGoStruct(t *testing.T) {
	cfg := DefaultConfig()
	schema := jsonschema.Reflect(&Config{})

	tests := []struct {
		section  string
		field    string
		expected interface{}
	}{
		{"GeneralConfig", "root", strVal(cfg.General.Root)},
		{"GeneralConfig", "library_dir", strVal(cfg.General.LibraryDir)},
		{"GeneralConfig", "log_level", strVal(cfg.General.LogLevel)},
		{"LLMConfig", "provider", strVal(cfg.LLM.Provider)},
		{"LLMConfig", "model", strVal(cfg.LLM.Model)},
		{"LLMConfig", "confidence_threshold", floatVal(cfg.LLM.ConfidenceThreshold)},
		{"LLMConfig", "temperature", floatVal(cfg.LLM.Temperature)},
		{"ApplyConfig", "compression", strVal(cfg.Apply.Compression)},
		{"ApplyConfig", "conflict", strVal(cfg.Apply.Conflict)},
		{"KavitaConfig", "port", intVal(cfg.Kavita.Port)},
	}

	for _, tt := range tests {
		t.Run(tt.section+"."+tt.field, func(t *testing.T) {
			def, ok := schema.Definitions[tt.section]
			if !ok {
				t.Fatalf("Definition %s not found", tt.section)
			}
			prop, ok := def.Properties.Get(tt.field)
			if !ok {
				t.Fatalf("Property %s not found in %s", tt.field, tt.section)
			}
			if prop.Default == nil {
				t.Errorf("%s.%s has no default in schema", tt.section, tt.field)
				return
			}

			expectedStr := stringify(tt.expected)
			actualStr := stringify(prop.Default)
			if expectedStr != actualStr {
				t.Errorf("%s.%s default = %v, want %v", tt.section, tt.field, prop.Default, tt.expected)
			}
		})
	}
}

func stringify(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
