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
	// StrPtr/StrVal
	s := StrPtr("hello")
	if StrVal(s) != "hello" {
		t.Errorf("StrVal(StrPtr('hello')) = %q", StrVal(s))
	}
	if StrVal(nil) != "" {
		t.Errorf("StrVal(nil) = %q, want empty", StrVal(nil))
	}

	// BoolPtr/BoolVal
	b := BoolPtr(true)
	if !BoolVal(b) {
		t.Error("BoolVal(BoolPtr(true)) should be true")
	}
	if BoolVal(nil) {
		t.Error("BoolVal(nil) should be false")
	}

	// IntPtr/IntVal
	i := IntPtr(42)
	if IntVal(i) != 42 {
		t.Errorf("IntVal(IntPtr(42)) = %d", IntVal(i))
	}
	if IntVal(nil) != 0 {
		t.Errorf("IntVal(nil) = %d, want 0", IntVal(nil))
	}

	// FloatPtr/FloatVal
	f := FloatPtr(3.14)
	if FloatVal(f) != 3.14 {
		t.Errorf("FloatVal(FloatPtr(3.14)) = %f", FloatVal(f))
	}
	if FloatVal(nil) != 0 {
		t.Errorf("FloatVal(nil) = %f, want 0", FloatVal(nil))
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
	if StrVal(cfg.General.Root) != "~" {
		t.Errorf("General.Root = %q, want ~", StrVal(cfg.General.Root))
	}
	if StrVal(cfg.General.LibraryDir) != "~/Library" {
		t.Errorf("General.LibraryDir = %q, want ~/Library", StrVal(cfg.General.LibraryDir))
	}
	if StrVal(cfg.General.ManifestPath) != "~/.local/share/lfx/manifest.db" {
		t.Errorf("General.ManifestPath = %q", StrVal(cfg.General.ManifestPath))
	}
	if StrVal(cfg.General.LogLevel) != "info" {
		t.Errorf("General.LogLevel = %q, want info", StrVal(cfg.General.LogLevel))
	}
	if cfg.General.DryRun == nil || !*cfg.General.DryRun {
		t.Error("General.DryRun should default to true")
	}

	// Scan
	if cfg.Scan == nil {
		t.Fatal("Scan config is nil")
	}
	if IntVal(cfg.Scan.MaxDepth) != 5 {
		t.Errorf("Scan.MaxDepth = %d, want 5", IntVal(cfg.Scan.MaxDepth))
	}
	if IntVal(cfg.Scan.MinFileSizeKB) != 10 {
		t.Errorf("Scan.MinFileSizeKB = %d, want 10", IntVal(cfg.Scan.MinFileSizeKB))
	}
	if IntVal(cfg.Scan.MaxFileSizeMB) != 500 {
		t.Errorf("Scan.MaxFileSizeMB = %d, want 500", IntVal(cfg.Scan.MaxFileSizeMB))
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
	if StrVal(cfg.LLM.Provider) != "ollama" {
		t.Errorf("LLM.Provider = %q, want ollama", StrVal(cfg.LLM.Provider))
	}
	if StrVal(cfg.LLM.Model) != "qwen3:8b" {
		t.Errorf("LLM.Model = %q, want qwen3:8b", StrVal(cfg.LLM.Model))
	}
	if StrVal(cfg.LLM.FallbackModel) != "llama3.3:8b" {
		t.Errorf("LLM.FallbackModel = %q, want llama3.3:8b", StrVal(cfg.LLM.FallbackModel))
	}
	if FloatVal(cfg.LLM.ConfidenceThreshold) != 0.7 {
		t.Errorf("LLM.ConfidenceThreshold = %f, want 0.7", FloatVal(cfg.LLM.ConfidenceThreshold))
	}
	if FloatVal(cfg.LLM.ConfidenceWarning) != 0.9 {
		t.Errorf("LLM.ConfidenceWarning = %f, want 0.9", FloatVal(cfg.LLM.ConfidenceWarning))
	}
	if FloatVal(cfg.LLM.Temperature) != 0.2 {
		t.Errorf("LLM.Temperature = %f, want 0.2", FloatVal(cfg.LLM.Temperature))
	}
	if IntVal(cfg.LLM.MaxConcurrent) != 3 {
		t.Errorf("LLM.MaxConcurrent = %d, want 3", IntVal(cfg.LLM.MaxConcurrent))
	}
	if IntVal(cfg.LLM.TimeoutSeconds) != 60 {
		t.Errorf("LLM.TimeoutSeconds = %d, want 60", IntVal(cfg.LLM.TimeoutSeconds))
	}
	if cfg.LLM.Retry == nil || !*cfg.LLM.Retry {
		t.Error("LLM.Retry should default to true")
	}
	if IntVal(cfg.LLM.MaxRetries) != 2 {
		t.Errorf("LLM.MaxRetries = %d, want 2", IntVal(cfg.LLM.MaxRetries))
	}

	// Library
	if cfg.Library == nil {
		t.Fatal("Library config is nil")
	}
	if StrVal(cfg.Library.Layout) != "{tipo}/{subject}" {
		t.Errorf("Library.Layout = %q, want {tipo}/{subject}", StrVal(cfg.Library.Layout))
	}
	if cfg.Library.Types == nil || len(cfg.Library.Types.ValidTypes) != 8 {
		t.Errorf("Library.Types.ValidTypes should have 8 types")
	}
	if cfg.Library.Subjects == nil || StrVal(cfg.Library.Subjects.Strategy) != "emergente" {
		t.Errorf("Library.Subjects.Strategy = %q, want emergente", StrVal(cfg.Library.Subjects.Strategy))
	}

	// Metadata
	if cfg.Metadata == nil {
		t.Fatal("Metadata config is nil")
	}
	if StrVal(cfg.Metadata.EbookMetaPath) != "ebook-meta" {
		t.Errorf("Metadata.EbookMetaPath = %q, want ebook-meta", StrVal(cfg.Metadata.EbookMetaPath))
	}

	// OCR
	if cfg.OCR == nil {
		t.Fatal("OCR config is nil")
	}
	if IntVal(cfg.OCR.MaxConcurrent) != 2 {
		t.Errorf("OCR.MaxConcurrent = %d, want 2", IntVal(cfg.OCR.MaxConcurrent))
	}
	if cfg.OCR.Quality == nil {
		t.Fatal("OCR.Quality is nil")
	}
	if FloatVal(cfg.OCR.Quality.BlurThreshold) != 100.0 {
		t.Errorf("OCR.Quality.BlurThreshold = %f, want 100.0", FloatVal(cfg.OCR.Quality.BlurThreshold))
	}

	// Apply
	if cfg.Apply == nil {
		t.Fatal("Apply config is nil")
	}
	if StrVal(cfg.Apply.FilenameTemplate) != "{autor} - {titulo} ({anio})" {
		t.Errorf("Apply.FilenameTemplate = %q", StrVal(cfg.Apply.FilenameTemplate))
	}
	if StrVal(cfg.Apply.Compression) != "ocrmypdf" {
		t.Errorf("Apply.Compression = %q, want ocrmypdf", StrVal(cfg.Apply.Compression))
	}
	if StrVal(cfg.Apply.Conflict) != "suffix" {
		t.Errorf("Apply.Conflict = %q, want suffix", StrVal(cfg.Apply.Conflict))
	}
	if cfg.Apply.BackupCompress == nil || !*cfg.Apply.BackupCompress {
		t.Error("Apply.BackupCompress should default to true")
	}
	if IntVal(cfg.Apply.BackupMaxCount) != 5 {
		t.Errorf("Apply.BackupMaxCount = %d, want 5", IntVal(cfg.Apply.BackupMaxCount))
	}

	// Kavita
	if cfg.Kavita == nil {
		t.Fatal("Kavita config is nil")
	}
	if cfg.Kavita.Enabled != nil && *cfg.Kavita.Enabled {
		t.Error("Kavita.Enabled should default to false")
	}
	if IntVal(cfg.Kavita.Port) != 5000 {
		t.Errorf("Kavita.Port = %d, want 5000", IntVal(cfg.Kavita.Port))
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
		Root:         StrPtr("~"),
		LibraryDir:   StrPtr("~/Library"),
		ManifestPath: StrPtr("~/.local/share/lfx/manifest.db"),
		LogLevel:     StrPtr("info"),
		DryRun:       BoolPtr(true),
	}
	src := &GeneralConfig{
		Root:     StrPtr("/custom/root"),
		LogLevel: StrPtr("debug"),
	}

	mergeGeneral(dst, src)

	if StrVal(dst.Root) != "/custom/root" {
		t.Errorf("Root = %q, want /custom/root", StrVal(dst.Root))
	}
	if StrVal(dst.LibraryDir) != "~/Library" {
		t.Errorf("LibraryDir should not change, got %q", StrVal(dst.LibraryDir))
	}
	if StrVal(dst.LogLevel) != "debug" {
		t.Errorf("LogLevel = %q, want debug", StrVal(dst.LogLevel))
	}
	if dst.DryRun == nil || !*dst.DryRun {
		t.Error("DryRun should remain true")
	}
}

func TestMergeGeneral_DryRunOverride(t *testing.T) {
	dst := &GeneralConfig{
		Root:   StrPtr("~"),
		DryRun: BoolPtr(true),
	}
	src := &GeneralConfig{
		DryRun: BoolPtr(false),
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
		Root:   StrPtr("~"),
		DryRun: BoolPtr(true),
	}
	src := &GeneralConfig{
		Root: StrPtr("/new/root"),
	}

	mergeGeneral(dst, src)

	if dst.DryRun == nil || !*dst.DryRun {
		t.Error("DryRun should remain true when src.DryRun is nil")
	}
}

func TestMergeScan(t *testing.T) {
	dst := &ScanConfig{
		MaxDepth:      IntPtr(5),
		MinFileSizeKB: IntPtr(10),
		MaxFileSizeMB: IntPtr(500),
		Extensions:    []string{"pdf"},
		ExcludeDirs:   []string{"Library"},
	}
	src := &ScanConfig{
		MaxDepth:      IntPtr(3),
		MinFileSizeKB: IntPtr(20),
		MaxFileSizeMB: IntPtr(100),
		Extensions:    []string{"pdf", "djvu"},
		ExcludeDirs:   []string{"node_modules"},
	}

	mergeScan(dst, src)

	if IntVal(dst.MaxDepth) != 3 {
		t.Errorf("MaxDepth = %d, want 3", IntVal(dst.MaxDepth))
	}
	if IntVal(dst.MinFileSizeKB) != 20 {
		t.Errorf("MinFileSizeKB = %d, want 20", IntVal(dst.MinFileSizeKB))
	}
	if IntVal(dst.MaxFileSizeMB) != 100 {
		t.Errorf("MaxFileSizeMB = %d, want 100", IntVal(dst.MaxFileSizeMB))
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
		Provider:            StrPtr("ollama"),
		Model:               StrPtr("qwen3:8b"),
		FallbackModel:       StrPtr("llama3.3:8b"),
		ConfidenceThreshold: FloatPtr(0.7),
		ConfidenceWarning:   FloatPtr(0.9),
		Temperature:         FloatPtr(0.2),
		MaxConcurrent:       IntPtr(3),
		TimeoutSeconds:      IntPtr(60),
		LocalOnlyCategories: []string{"Personal"},
		Retry:               BoolPtr(true),
		MaxRetries:          IntPtr(2),
	}
	src := &LLMConfig{
		Provider:            StrPtr("groq"),
		Model:               StrPtr("llama3.1:8b"),
		FallbackModel:       StrPtr("qwen3:8b"),
		ConfidenceThreshold: FloatPtr(0.8),
		ConfidenceWarning:   FloatPtr(0.95),
		Temperature:         FloatPtr(0.5),
		MaxConcurrent:       IntPtr(5),
		TimeoutSeconds:      IntPtr(120),
		LocalOnlyCategories: []string{"Admin"},
		Retry:               BoolPtr(false),
		MaxRetries:          IntPtr(3),
	}

	mergeLLM(dst, src)

	if StrVal(dst.Provider) != "groq" {
		t.Errorf("Provider = %q, want groq", StrVal(dst.Provider))
	}
	if StrVal(dst.Model) != "llama3.1:8b" {
		t.Errorf("Model = %q, want llama3.1:8b", StrVal(dst.Model))
	}
	if StrVal(dst.FallbackModel) != "qwen3:8b" {
		t.Errorf("FallbackModel = %q, want qwen3:8b", StrVal(dst.FallbackModel))
	}
	if FloatVal(dst.ConfidenceThreshold) != 0.8 {
		t.Errorf("ConfidenceThreshold = %f, want 0.8", FloatVal(dst.ConfidenceThreshold))
	}
	if FloatVal(dst.ConfidenceWarning) != 0.95 {
		t.Errorf("ConfidenceWarning = %f, want 0.95", FloatVal(dst.ConfidenceWarning))
	}
	if FloatVal(dst.Temperature) != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", FloatVal(dst.Temperature))
	}
	if IntVal(dst.MaxConcurrent) != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", IntVal(dst.MaxConcurrent))
	}
	if IntVal(dst.TimeoutSeconds) != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", IntVal(dst.TimeoutSeconds))
	}
	if dst.LocalOnlyCategories[0] != "Admin" {
		t.Errorf("LocalOnlyCategories = %v, want [Admin]", dst.LocalOnlyCategories)
	}
	if dst.Retry == nil || *dst.Retry {
		t.Error("Retry should be false")
	}
	if IntVal(dst.MaxRetries) != 3 {
		t.Errorf("MaxRetries = %d, want 3", IntVal(dst.MaxRetries))
	}
}

func TestMergeLibrary(t *testing.T) {
	dst := &LibraryConfig{
		Layout: StrPtr("{tipo}/{subject}"),
		Types: &LibraryTypes{
			ValidTypes: []string{"Libro", "Paper"},
		},
		Subjects: &LibrarySubjects{
			Strategy:       StrPtr("emergente"),
			MergeThreshold: FloatPtr(0.8),
		},
	}
	src := &LibraryConfig{
		Layout: StrPtr("{subject}/{tipo}"),
		Subjects: &LibrarySubjects{
			MergeThreshold: FloatPtr(0.9),
		},
	}

	mergeLibrary(dst, src)

	if StrVal(dst.Layout) != "{subject}/{tipo}" {
		t.Errorf("Layout = %q, want {subject}/{tipo}", StrVal(dst.Layout))
	}
	if len(dst.Types.ValidTypes) != 2 {
		t.Errorf("Types.ValidTypes should not change, got %v", dst.Types.ValidTypes)
	}
	if StrVal(dst.Subjects.Strategy) != "emergente" {
		t.Errorf("Strategy should not change, got %q", StrVal(dst.Subjects.Strategy))
	}
	if FloatVal(dst.Subjects.MergeThreshold) != 0.9 {
		t.Errorf("MergeThreshold = %f, want 0.9", FloatVal(dst.Subjects.MergeThreshold))
	}
}

func TestMergeLibrary_StrategyOverride(t *testing.T) {
	dst := &LibraryConfig{
		Subjects: &LibrarySubjects{
			Strategy: StrPtr("emergente"),
		},
	}
	src := &LibraryConfig{
		Subjects: &LibrarySubjects{
			Strategy: StrPtr("predefinida"),
		},
	}

	mergeLibrary(dst, src)

	if StrVal(dst.Subjects.Strategy) != "predefinida" {
		t.Errorf("Strategy = %q, want predefinida", StrVal(dst.Subjects.Strategy))
	}
}

func TestMergeLibrary_NilSections(t *testing.T) {
	dst := &LibraryConfig{
		Layout: StrPtr("{tipo}/{subject}"),
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
		EbookMetaPath:     StrPtr("ebook-meta"),
		FetchMetadataPath: StrPtr("fetch-ebook-metadata"),
		MetadataSources:   []string{"google_books"},
	}
	src := &MetadataConfig{
		MetadataSources: []string{"open_library", "amazon"},
	}

	mergeMetadata(dst, src)

	if StrVal(dst.EbookMetaPath) != "ebook-meta" {
		t.Errorf("EbookMetaPath should not change, got %q", StrVal(dst.EbookMetaPath))
	}
	if StrVal(dst.FetchMetadataPath) != "fetch-ebook-metadata" {
		t.Errorf("FetchMetadataPath should not change, got %q", StrVal(dst.FetchMetadataPath))
	}
	if len(dst.MetadataSources) != 2 {
		t.Errorf("MetadataSources should be overwritten, got %v", dst.MetadataSources)
	}
}

func TestMergeMetadata_PathOverrides(t *testing.T) {
	dst := &MetadataConfig{
		EbookMetaPath:     StrPtr("ebook-meta"),
		FetchMetadataPath: StrPtr("fetch-ebook-metadata"),
	}
	src := &MetadataConfig{
		EbookMetaPath:     StrPtr("/usr/local/bin/ebook-meta"),
		FetchMetadataPath: StrPtr("/usr/local/bin/fetch-ebook-metadata"),
	}

	mergeMetadata(dst, src)

	if StrVal(dst.EbookMetaPath) != "/usr/local/bin/ebook-meta" {
		t.Errorf("EbookMetaPath = %q, want /usr/local/bin/ebook-meta", StrVal(dst.EbookMetaPath))
	}
	if StrVal(dst.FetchMetadataPath) != "/usr/local/bin/fetch-ebook-metadata" {
		t.Errorf("FetchMetadataPath = %q, want /usr/local/bin/fetch-ebook-metadata", StrVal(dst.FetchMetadataPath))
	}
}

func TestMergeOCR(t *testing.T) {
	dst := &OCRConfig{
		Languages:     []string{"eng"},
		MaxConcurrent: IntPtr(2),
		Quality: &OCRQuality{
			BlurThreshold:     FloatPtr(100.0),
			SkewThreshold:     FloatPtr(2.0),
			ContrastMinSpread: FloatPtr(30.0),
		},
	}
	src := &OCRConfig{
		Languages: []string{"spa", "deu"},
		Quality: &OCRQuality{
			BlurThreshold: FloatPtr(150.0),
		},
	}

	mergeOCR(dst, src)

	if len(dst.Languages) != 2 || dst.Languages[0] != "spa" {
		t.Errorf("Languages = %v, want [spa deu]", dst.Languages)
	}
	if IntVal(dst.MaxConcurrent) != 2 {
		t.Errorf("MaxConcurrent should not change, got %d", IntVal(dst.MaxConcurrent))
	}
	if FloatVal(dst.Quality.BlurThreshold) != 150.0 {
		t.Errorf("BlurThreshold = %f, want 150.0", FloatVal(dst.Quality.BlurThreshold))
	}
	if FloatVal(dst.Quality.SkewThreshold) != 2.0 {
		t.Errorf("SkewThreshold should not change, got %f", FloatVal(dst.Quality.SkewThreshold))
	}
}

func TestMergeOCR_MaxConcurrentOverride(t *testing.T) {
	dst := &OCRConfig{
		MaxConcurrent: IntPtr(2),
	}
	src := &OCRConfig{
		MaxConcurrent: IntPtr(4),
	}

	mergeOCR(dst, src)

	if IntVal(dst.MaxConcurrent) != 4 {
		t.Errorf("MaxConcurrent = %d, want 4", IntVal(dst.MaxConcurrent))
	}
}

func TestMergeOCR_NilQuality(t *testing.T) {
	dst := &OCRConfig{
		Languages: []string{"eng"},
	}
	src := &OCRConfig{
		Quality: &OCRQuality{
			BlurThreshold: FloatPtr(150.0),
		},
	}

	mergeOCR(dst, src)

	if dst.Quality == nil {
		t.Fatal("Quality should be created from src")
	}
	if FloatVal(dst.Quality.BlurThreshold) != 150.0 {
		t.Errorf("BlurThreshold = %f, want 150.0", FloatVal(dst.Quality.BlurThreshold))
	}
}

func TestMergeApply(t *testing.T) {
	dst := &ApplyConfig{
		FilenameTemplate: StrPtr("{autor} - {titulo} ({anio})"),
		Compression:      StrPtr("ocrmypdf"),
		Conflict:         StrPtr("suffix"),
		BackupDir:        StrPtr("{library_dir}/.backups"),
		BackupCompress:   BoolPtr(true),
		BackupMaxCount:   IntPtr(5),
		BackupMaxAgeDays: IntPtr(30),
	}
	src := &ApplyConfig{
		FilenameTemplate: StrPtr("{titulo} - {autor}.pdf"),
		Compression:      StrPtr("ghostscript"),
		Conflict:         StrPtr("overwrite"),
		BackupDir:        StrPtr("/backups"),
		BackupCompress:   BoolPtr(false),
		BackupMaxCount:   IntPtr(10),
		BackupMaxAgeDays: IntPtr(60),
	}

	mergeApply(dst, src)

	if StrVal(dst.FilenameTemplate) != "{titulo} - {autor}.pdf" {
		t.Errorf("FilenameTemplate = %q", StrVal(dst.FilenameTemplate))
	}
	if StrVal(dst.Compression) != "ghostscript" {
		t.Errorf("Compression = %q, want ghostscript", StrVal(dst.Compression))
	}
	if StrVal(dst.Conflict) != "overwrite" {
		t.Errorf("Conflict = %q, want overwrite", StrVal(dst.Conflict))
	}
	if StrVal(dst.BackupDir) != "/backups" {
		t.Errorf("BackupDir = %q, want /backups", StrVal(dst.BackupDir))
	}
	if dst.BackupCompress == nil || *dst.BackupCompress {
		t.Error("BackupCompress should be false")
	}
	if IntVal(dst.BackupMaxCount) != 10 {
		t.Errorf("BackupMaxCount = %d, want 10", IntVal(dst.BackupMaxCount))
	}
	if IntVal(dst.BackupMaxAgeDays) != 60 {
		t.Errorf("BackupMaxAgeDays = %d, want 60", IntVal(dst.BackupMaxAgeDays))
	}
}

func TestMergeKavita(t *testing.T) {
	dst := &KavitaConfig{
		Enabled:           BoolPtr(false),
		DockerComposePath: StrPtr("~/.config/lfx/docker-compose.kavita.yml"),
		Port:              IntPtr(5000),
	}
	src := &KavitaConfig{
		Enabled: BoolPtr(true),
		Port:    IntPtr(8080),
	}

	mergeKavita(dst, src)

	if dst.Enabled == nil || !*dst.Enabled {
		t.Error("Enabled should be true")
	}
	if IntVal(dst.Port) != 8080 {
		t.Errorf("Port = %d, want 8080", IntVal(dst.Port))
	}
	if StrVal(dst.DockerComposePath) != "~/.config/lfx/docker-compose.kavita.yml" {
		t.Errorf("DockerComposePath should not change, got %q", StrVal(dst.DockerComposePath))
	}
}

func TestMergeFullConfig(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		General: &GeneralConfig{
			Root:     StrPtr("/custom"),
			LogLevel: StrPtr("debug"),
		},
		LLM: &LLMConfig{
			Provider: StrPtr("groq"),
		},
	}

	merge(dst, src)

	if StrVal(dst.General.Root) != "/custom" {
		t.Errorf("General.Root = %q, want /custom", StrVal(dst.General.Root))
	}
	if StrVal(dst.LLM.Provider) != "groq" {
		t.Errorf("LLM.Provider = %q, want groq", StrVal(dst.LLM.Provider))
	}
	// Preserved defaults
	if StrVal(dst.LLM.Model) != "qwen3:8b" {
		t.Errorf("LLM.Model should be preserved, got %q", StrVal(dst.LLM.Model))
	}
}

func TestMergeAllSections(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		Library: &LibraryConfig{
			Layout: StrPtr("{subject}/{tipo}"),
		},
		Metadata: &MetadataConfig{
			MetadataSources: []string{"amazon"},
		},
		OCR: &OCRConfig{
			Languages: []string{"deu"},
		},
		Apply: &ApplyConfig{
			Compression: StrPtr("ghostscript"),
		},
		Kavita: &KavitaConfig{
			Enabled: BoolPtr(true),
			Port:    IntPtr(8080),
		},
	}

	merge(dst, src)

	if StrVal(dst.Library.Layout) != "{subject}/{tipo}" {
		t.Errorf("Library.Layout = %q", StrVal(dst.Library.Layout))
	}
	if dst.Metadata.MetadataSources[0] != "amazon" {
		t.Errorf("Metadata.MetadataSources = %v", dst.Metadata.MetadataSources)
	}
	if dst.OCR.Languages[0] != "deu" {
		t.Errorf("OCR.Languages = %v", dst.OCR.Languages)
	}
	if StrVal(dst.Apply.Compression) != "ghostscript" {
		t.Errorf("Apply.Compression = %q", StrVal(dst.Apply.Compression))
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

	if StrVal(cfg.General.Root) != "/tmp/test-root" {
		t.Errorf("General.Root = %q, want /tmp/test-root", StrVal(cfg.General.Root))
	}
	if StrVal(cfg.General.LogLevel) != "debug" {
		t.Errorf("General.LogLevel = %q, want debug", StrVal(cfg.General.LogLevel))
	}
	if StrVal(cfg.LLM.Provider) != "groq" {
		t.Errorf("LLM.Provider = %q, want groq", StrVal(cfg.LLM.Provider))
	}
	// Defaults should be preserved
	if FloatVal(cfg.LLM.ConfidenceThreshold) != 0.7 {
		t.Errorf("LLM.ConfidenceThreshold = %f, want 0.7 (default preserved)", FloatVal(cfg.LLM.ConfidenceThreshold))
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
				LLM: &LLMConfig{Provider: StrPtr("ollama")},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: &Config{
				General: &GeneralConfig{
					Root:         StrPtr("~"),
					LibraryDir:   StrPtr("~/Library"),
					ManifestPath: StrPtr("~/.local/share/lfx/manifest.db"),
					LogLevel:     StrPtr("invalid"),
					DryRun:       BoolPtr(true),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid provider",
			cfg: &Config{
				General: &GeneralConfig{
					Root:         StrPtr("~"),
					LibraryDir:   StrPtr("~/Library"),
					ManifestPath: StrPtr("~/.local/share/lfx/manifest.db"),
					LogLevel:     StrPtr("info"),
					DryRun:       BoolPtr(true),
				},
				LLM: &LLMConfig{
					Provider: StrPtr("openai"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid compression",
			cfg: &Config{
				General: &GeneralConfig{
					Root:         StrPtr("~"),
					LibraryDir:   StrPtr("~/Library"),
					ManifestPath: StrPtr("~/.local/share/lfx/manifest.db"),
					LogLevel:     StrPtr("info"),
					DryRun:       BoolPtr(true),
				},
				Apply: &ApplyConfig{
					Compression: StrPtr("invalid"),
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
			Root:         StrPtr(""),
			LibraryDir:   StrPtr(""),
			ManifestPath: StrPtr(""),
			LogLevel:     StrPtr("info"),
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should fail with empty required fields")
	}
}

func TestValidate_OutOfRangeValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.ConfidenceThreshold = FloatPtr(1.5)
	cfg.LLM.Temperature = FloatPtr(3.0)

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should fail with out-of-range values")
	}
}

func TestValidate_InvalidConflict(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Apply.Conflict = StrPtr("invalid")

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
		{"GeneralConfig", "root", StrVal(cfg.General.Root)},
		{"GeneralConfig", "library_dir", StrVal(cfg.General.LibraryDir)},
		{"GeneralConfig", "log_level", StrVal(cfg.General.LogLevel)},
		{"LLMConfig", "provider", StrVal(cfg.LLM.Provider)},
		{"LLMConfig", "model", StrVal(cfg.LLM.Model)},
		{"LLMConfig", "confidence_threshold", FloatVal(cfg.LLM.ConfidenceThreshold)},
		{"LLMConfig", "temperature", FloatVal(cfg.LLM.Temperature)},
		{"ApplyConfig", "compression", StrVal(cfg.Apply.Compression)},
		{"ApplyConfig", "conflict", StrVal(cfg.Apply.Conflict)},
		{"KavitaConfig", "port", IntVal(cfg.Kavita.Port)},
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

func TestResolveTemplateVars(t *testing.T) {
	tests := []struct {
		name          string
		libraryDir    string
		backupDir     string
		wantBackupDir string
	}{
		{
			name:          "resolves library_dir",
			libraryDir:    "/Users/test/Biblioteca",
			backupDir:     "{library_dir}/.backups",
			wantBackupDir: "/Users/test/Biblioteca/.backups",
		},
		{
			name:          "resolves root",
			libraryDir:    "/Users/test/Biblioteca",
			backupDir:     "{root}/backups",
			wantBackupDir: "~/backups", // Root is "~", not expanded
		},
		{
			name:          "no template vars",
			libraryDir:    "/Users/test/Biblioteca",
			backupDir:     "/absolute/path/.backups",
			wantBackupDir: "/absolute/path/.backups",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				General: &GeneralConfig{
					Root:       StrPtr("~"),
					LibraryDir: StrPtr(tt.libraryDir),
				},
				Apply: &ApplyConfig{
					BackupDir: StrPtr(tt.backupDir),
				},
			}
			resolveTemplateVars(cfg)
			got := StrVal(cfg.Apply.BackupDir)
			if got != tt.wantBackupDir {
				t.Errorf("BackupDir = %q, want %q", got, tt.wantBackupDir)
			}
		})
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestLoadFrom_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.toml")

	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom(empty) error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFrom(empty) returned nil")
	}
	// Should use all defaults
	if StrVal(cfg.General.LogLevel) != "info" {
		t.Errorf("LogLevel = %q, want info (default)", StrVal(cfg.General.LogLevel))
	}
}

func TestLoadFrom_UnknownKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unknown.toml")

	content := `
[general]
log_level = "debug"
unknown_key = "should be ignored"
another_unknown = 123

[llm]
provider = "groq"
totally_fake_field = true
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom(unknown keys) error = %v", err)
	}
	// Known keys should be set
	if StrVal(cfg.General.LogLevel) != "debug" {
		t.Errorf("LogLevel = %q, want debug", StrVal(cfg.General.LogLevel))
	}
	if StrVal(cfg.LLM.Provider) != "groq" {
		t.Errorf("Provider = %q, want groq", StrVal(cfg.LLM.Provider))
	}
	// Unknown keys should be silently ignored (BurntSushi/toml behavior)
}

func TestLoadFrom_PartialSections(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.toml")

	content := `
[llm]
model = "llama3.3:8b"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom(partial) error = %v", err)
	}
	// Only model should be overridden
	if StrVal(cfg.LLM.Model) != "llama3.3:8b" {
		t.Errorf("Model = %q, want llama3.3:8b", StrVal(cfg.LLM.Model))
	}
	// Other LLM fields should keep defaults
	if StrVal(cfg.LLM.Provider) != "ollama" {
		t.Errorf("Provider = %q, want ollama (default)", StrVal(cfg.LLM.Provider))
	}
	if FloatVal(cfg.LLM.ConfidenceThreshold) != 0.7 {
		t.Errorf("ConfidenceThreshold = %f, want 0.7 (default)", FloatVal(cfg.LLM.ConfidenceThreshold))
	}
}

func TestLoadFrom_WrongTypes(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wrongtype.toml")

	// dry_run expects bool, we give string
	content := `
[general]
dry_run = "si"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(configPath)
	if err == nil {
		t.Fatal("LoadFrom(wrong type) should return error")
	}
	// BurntSushi/toml returns an error for type mismatches
	t.Logf("Got expected error: %v", err)
}

func TestLoadFrom_OutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "oorange.toml")

	content := `
[llm]
confidence_threshold = 2.0
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(configPath)
	if err == nil {
		t.Fatal("LoadFrom(out of range) should return error from Validate()")
	}
	t.Logf("Got expected error: %v", err)
}

func TestLoadFrom_GlobalLocalOverride(t *testing.T) {
	// Simulate global config
	home, _ := os.UserHomeDir()
	globalDir := filepath.Join(home, ".config", "lfx")
	globalPath := filepath.Join(globalDir, "lfx.toml")

	// Create global config
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Skip("Cannot create global config dir")
	}

	globalContent := `
[general]
log_level = "warn"

[llm]
provider = "groq"
`
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(globalPath) }() // cleanup

	// Create local config
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "lfx.toml")
	localContent := `
[general]
log_level = "debug"
`
	if err := os.WriteFile(localPath, []byte(localContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load global first, then local should override
	cfg := DefaultConfig()
	if err := mergeFile(cfg, globalPath); err != nil {
		t.Fatal(err)
	}
	if err := mergeFile(cfg, localPath); err != nil {
		t.Fatal(err)
	}

	// Local should win for log_level
	if StrVal(cfg.General.LogLevel) != "debug" {
		t.Errorf("LogLevel = %q, want debug (local override)", StrVal(cfg.General.LogLevel))
	}
	// Global should remain for provider (not overridden by local)
	if StrVal(cfg.LLM.Provider) != "groq" {
		t.Errorf("Provider = %q, want groq (from global)", StrVal(cfg.LLM.Provider))
	}
}

func TestLoadFrom_NonExistentFile(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("LoadFrom(nonexistent) should return error")
	}
	t.Logf("Got expected error: %v", err)
}

func TestLoadFrom_NestedPartial(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested.toml")

	content := `
[ocr]
max_concurrent = 4

[ocr.quality]
blur_threshold = 200.0
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom(nested partial) error = %v", err)
	}
	// Overridden values
	if IntVal(cfg.OCR.MaxConcurrent) != 4 {
		t.Errorf("MaxConcurrent = %d, want 4", IntVal(cfg.OCR.MaxConcurrent))
	}
	if FloatVal(cfg.OCR.Quality.BlurThreshold) != 200.0 {
		t.Errorf("BlurThreshold = %f, want 200.0", FloatVal(cfg.OCR.Quality.BlurThreshold))
	}
	// Non-overridden values should keep defaults
	if FloatVal(cfg.OCR.Quality.SkewThreshold) != 2.0 {
		t.Errorf("SkewThreshold = %f, want 2.0 (default)", FloatVal(cfg.OCR.Quality.SkewThreshold))
	}
}
