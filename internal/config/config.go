// Package config handles loading, validating, and merging lfx configuration.
//
// Configuration priority: defaults < ~/.config/lfx/lfx.toml < ./lfx.toml < --flags < $ENV_VARS
//
//go:generate go run ../../cmd/genschema
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ============================================================================
// Pointer helpers — Go lacks syntactic sugar for pointer creation
// ============================================================================

func strPtr(s string) *string     { return &s }
func boolPtr(b bool) *bool        { return &b }
func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolVal(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func intVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func floatVal(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// Config is the root configuration structure for lfx.
type Config struct {
	General  *GeneralConfig  `json:"general,omitempty" toml:"general"`
	Scan     *ScanConfig     `json:"scan,omitempty" toml:"scan"`
	LLM      *LLMConfig      `json:"llm,omitempty" toml:"llm"`
	Library  *LibraryConfig  `json:"library,omitempty" toml:"library"`
	Metadata *MetadataConfig `json:"metadata,omitempty" toml:"metadata"`
	OCR      *OCRConfig      `json:"ocr,omitempty" toml:"ocr"`
	Apply    *ApplyConfig    `json:"apply,omitempty" toml:"apply"`
	Kavita   *KavitaConfig   `json:"kavita,omitempty" toml:"kavita"`
}

// GeneralConfig holds core paths and behavior settings.
type GeneralConfig struct {
	Root         *string `json:"root,omitempty" toml:"root" jsonschema:"default=~,description=Root directory to scan for documents"`
	LibraryDir   *string `json:"library_dir,omitempty" toml:"library_dir" jsonschema:"default=~/Library,description=Destination directory for organized library"`
	ManifestPath *string `json:"manifest_path,omitempty" toml:"manifest_path" jsonschema:"default=~/.local/share/lfx/manifest.db,description=Path to the SQLite manifest database"`
	LogLevel     *string `json:"log_level,omitempty" toml:"log_level" jsonschema:"enum=debug,enum=info,enum=warn,enum=error,default=info,description=Logging verbosity level"`
	DryRun       *bool   `json:"dry_run,omitempty" toml:"dry_run" jsonschema:"default=true,description=If true, only report what would happen without modifying files"`
}

// ScanConfig holds file discovery and filtering settings.
type ScanConfig struct {
	MaxDepth      *int     `json:"max_depth,omitempty" toml:"max_depth" jsonschema:"default=5,minimum=1,maximum=20,description=Maximum directory recursion depth from root"`
	MinFileSizeKB *int     `json:"min_file_size_kb,omitempty" toml:"min_file_size_kb" jsonschema:"default=10,minimum=0,description=Ignore files smaller than this (in KB)"`
	MaxFileSizeMB *int     `json:"max_file_size_mb,omitempty" toml:"max_file_size_mb" jsonschema:"default=500,minimum=1,description=Ignore files larger than this (in MB)"`
	ExcludeDirs   []string `json:"exclude_dirs,omitempty" toml:"exclude_dirs" jsonschema:"description=Directory names to skip during scanning (symlinks always skipped)"`
	Extensions    []string `json:"extensions,omitempty" toml:"extensions" jsonschema:"description=File extensions to process (without the dot)"`
}

// LLMConfig holds LLM provider settings for document classification.
type LLMConfig struct {
	Provider            *string  `json:"provider,omitempty" toml:"provider" jsonschema:"enum=ollama,enum=groq,enum=cerebras,default=ollama,description=LLM backend to use"`
	Model               *string  `json:"model,omitempty" toml:"model" jsonschema:"default=qwen3:8b,description=Model name for the LLM provider"`
	FallbackModel       *string  `json:"fallback_model,omitempty" toml:"fallback_model" jsonschema:"default=llama3.3:8b,description=Fallback model if primary fails"`
	ConfidenceThreshold *float64 `json:"confidence_threshold,omitempty" toml:"confidence_threshold" jsonschema:"default=0.7,minimum=0.0,maximum=1.0,description=Minimum confidence for LLM classifications (below this -> unknown)"`
	ConfidenceWarning   *float64 `json:"confidence_warning,omitempty" toml:"confidence_warning" jsonschema:"default=0.9,minimum=0.0,maximum=1.0,description=Warning threshold for moderate confidence classifications"`
	Temperature         *float64 `json:"temperature,omitempty" toml:"temperature" jsonschema:"default=0.2,minimum=0.0,maximum=2.0,description=LLM temperature (lower = more deterministic)"`
	MaxConcurrent       *int     `json:"max_concurrent,omitempty" toml:"max_concurrent" jsonschema:"default=3,minimum=1,maximum=20,description=Maximum parallel LLM API calls"`
	TimeoutSeconds      *int     `json:"timeout_seconds,omitempty" toml:"timeout_seconds" jsonschema:"default=60,minimum=1,description=Timeout per LLM call in seconds"`
	LocalOnlyCategories []string `json:"local_only_categories,omitempty" toml:"local_only_categories" jsonschema:"description=Categories that must always be processed with local Ollama (never cloud)"`
	Retry               *bool    `json:"retry,omitempty" toml:"retry" jsonschema:"default=true,description=Enable retry with fallback model on low confidence"`
	MaxRetries          *int     `json:"max_retries,omitempty" toml:"max_retries" jsonschema:"default=2,minimum=0,maximum=5,description=Maximum retry attempts (same model + fallback)"`
}

// LibraryConfig holds library organization settings.
type LibraryConfig struct {
	Layout   *string          `json:"layout,omitempty" toml:"layout" jsonschema:"default={tipo}/{subject},description=Folder layout template for the organized library. WARNING: VS Code autocomplete mangles {curly braces} — type manually or verify after autocomplete."`
	Types    *LibraryTypes    `json:"types,omitempty" toml:"types"`
	Subjects *LibrarySubjects `json:"subjects,omitempty" toml:"subjects"`
}

// LibraryTypes holds document type definitions.
type LibraryTypes struct {
	ValidTypes []string `json:"valid_types,omitempty" toml:"valid_types" jsonschema:"description=Fixed document types the LLM can assign"`
}

// LibrarySubjects holds subject classification settings.
type LibrarySubjects struct {
	Strategy       *string  `json:"strategy,omitempty" toml:"strategy" jsonschema:"enum=emergente,enum=predefinida,enum=mixta,default=emergente,description=How subjects are determined"`
	MergeThreshold *float64 `json:"merge_threshold,omitempty" toml:"merge_threshold" jsonschema:"default=0.8,minimum=0.0,maximum=1.0,description=Similarity threshold to auto-group similar subjects"`
}

// MetadataConfig holds Calibre metadata lookup settings.
type MetadataConfig struct {
	EbookMetaPath     *string  `json:"ebook_meta_path,omitempty" toml:"ebook_meta_path" jsonschema:"default=ebook-meta,description=Path to Calibre's ebook-meta CLI"`
	FetchMetadataPath *string  `json:"fetch_metadata_path,omitempty" toml:"fetch_metadata_path" jsonschema:"default=fetch-ebook-metadata,description=Path to Calibre's fetch-ebook-metadata CLI"`
	MetadataSources   []string `json:"metadata_sources,omitempty" toml:"metadata_sources" jsonschema:"description=Metadata sources to cross-reference for books"`
}

// OCRConfig holds OCR settings for scanned documents.
type OCRConfig struct {
	Languages     []string    `json:"languages,omitempty" toml:"languages" jsonschema:"description=Tesseract language pack codes to use for OCR"`
	MaxConcurrent *int        `json:"max_concurrent,omitempty" toml:"max_concurrent" jsonschema:"default=2,minimum=1,maximum=8,description=Maximum parallel OCR jobs (CPU-intensive)"`
	Quality       *OCRQuality `json:"quality,omitempty" toml:"quality"`
}

// OCRQuality holds scan quality triage thresholds.
type OCRQuality struct {
	BlurThreshold     *float64 `json:"blur_threshold,omitempty" toml:"blur_threshold" jsonschema:"default=100.0,description=Laplacian variance below this = too blurry for direct OCR"`
	SkewThreshold     *float64 `json:"skew_threshold,omitempty" toml:"skew_threshold" jsonschema:"default=2.0,description=Maximum skew angle in degrees before deskew"`
	ContrastMinSpread *float64 `json:"contrast_min_spread,omitempty" toml:"contrast_min_spread" jsonschema:"default=30.0,description=Minimum histogram spread for acceptable contrast"`
}

// ApplyConfig holds settings for applying changes (move/rename/compress).
type ApplyConfig struct {
	FilenameTemplate *string `json:"filename_template,omitempty" toml:"filename_template" jsonschema:"default={autor} - {titulo} ({anio}),description=Output filename template using document metadata variables. WARNING: VS Code autocomplete mangles {curly braces} — type manually or verify after autocomplete."`
	Compression      *string `json:"compression,omitempty" toml:"compression" jsonschema:"enum=ocrmypdf,enum=ghostscript,enum=none,default=ocrmypdf,description=Compression method for PDFs"`
	Conflict         *string `json:"conflict,omitempty" toml:"conflict" jsonschema:"enum=suffix,enum=overwrite,enum=skip,enum=ask,default=suffix,description=How to handle filename conflicts in destination"`
	BackupDir        *string `json:"backup_dir,omitempty" toml:"backup_dir" jsonschema:"default={library_dir}/.backups,description=Where to store backups before apply. WARNING: VS Code autocomplete mangles {curly braces} — type manually or verify after autocomplete."`
	BackupCompress   *bool   `json:"backup_compress,omitempty" toml:"backup_compress" jsonschema:"default=true,description=Compress backup files (gzip)"`
	BackupMaxCount   *int    `json:"backup_max_count,omitempty" toml:"backup_max_count" jsonschema:"default=5,minimum=1,description=Maximum number of backups to keep"`
	BackupMaxAgeDays *int    `json:"backup_max_age_days,omitempty" toml:"backup_max_age_days" jsonschema:"default=30,minimum=1,description=Delete backups older than this (days)"`
}

// KavitaConfig holds Kavita server settings (Docker-based web reader + OPDS).
type KavitaConfig struct {
	Enabled           *bool   `json:"enabled,omitempty" toml:"enabled" jsonschema:"default=false,description=Enable Kavita server management"`
	DockerComposePath *string `json:"docker_compose_path,omitempty" toml:"docker_compose_path" jsonschema:"default=~/.config/lfx/docker-compose.kavita.yml,description=Path to the Kavita docker-compose.yml file"`
	Port              *int    `json:"port,omitempty" toml:"port" jsonschema:"default=5000,minimum=1,maximum=65535,description=Kavita web UI port"`
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		General: &GeneralConfig{
			Root:         strPtr("~"),
			LibraryDir:   strPtr("~/Library"),
			ManifestPath: strPtr("~/.local/share/lfx/manifest.db"),
			LogLevel:     strPtr("info"),
			DryRun:       boolPtr(true),
		},
		Scan: &ScanConfig{
			MaxDepth:      intPtr(5),
			MinFileSizeKB: intPtr(10),
			MaxFileSizeMB: intPtr(500),
			ExcludeDirs: []string{
				"Library", "node_modules", ".cache", ".Trash", ".npm", ".nvm",
				".cargo", ".rustup", "go/pkg", "go/src", ".local/share/Trash",
				"Applications", "Movies", "Music", "Pictures", "Public", ".git", "Desktop",
			},
			Extensions: []string{"pdf", "djvu", "docx", "doc"},
		},
		LLM: &LLMConfig{
			Provider:            strPtr("ollama"),
			Model:               strPtr("qwen3:8b"),
			FallbackModel:       strPtr("llama3.3:8b"),
			ConfidenceThreshold: floatPtr(0.7),
			ConfidenceWarning:   floatPtr(0.9),
			Temperature:         floatPtr(0.2),
			MaxConcurrent:       intPtr(3),
			TimeoutSeconds:      intPtr(60),
			LocalOnlyCategories: []string{"Personal / Administrativo"},
			Retry:               boolPtr(true),
			MaxRetries:          intPtr(2),
		},
		Library: &LibraryConfig{
			Layout: strPtr("{tipo}/{subject}"),
			Types: &LibraryTypes{
				ValidTypes: []string{
					"Libro", "Paper / Artículo", "Tesis", "Notas / Apuntes",
					"Manual / Guía", "Reporte / Doc. técnico", "Documento personal", "Otro",
				},
			},
			Subjects: &LibrarySubjects{
				Strategy:       strPtr("emergente"),
				MergeThreshold: floatPtr(0.8),
			},
		},
		Metadata: &MetadataConfig{
			EbookMetaPath:     strPtr("ebook-meta"),
			FetchMetadataPath: strPtr("fetch-ebook-metadata"),
			MetadataSources:   []string{"google_books", "open_library"},
		},
		OCR: &OCRConfig{
			Languages:     []string{"eng", "spa"},
			MaxConcurrent: intPtr(2),
			Quality: &OCRQuality{
				BlurThreshold:     floatPtr(100.0),
				SkewThreshold:     floatPtr(2.0),
				ContrastMinSpread: floatPtr(30.0),
			},
		},
		Apply: &ApplyConfig{
			FilenameTemplate: strPtr("{autor} - {titulo} ({anio})"),
			Compression:      strPtr("ocrmypdf"),
			Conflict:         strPtr("suffix"),
			BackupDir:        strPtr("{library_dir}/.backups"),
			BackupCompress:   boolPtr(true),
			BackupMaxCount:   intPtr(5),
			BackupMaxAgeDays: intPtr(30),
		},
		Kavita: &KavitaConfig{
			Enabled:           boolPtr(false),
			DockerComposePath: strPtr("~/.config/lfx/docker-compose.kavita.yml"),
			Port:              intPtr(5000),
		},
	}
}

// Load reads and merges configuration files, returning the final config.
// Priority: defaults < ~/.config/lfx/lfx.toml < ./lfx.toml
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load global config
	globalPath, err := globalConfigPath()
	if err != nil {
		return nil, fmt.Errorf("determining global config path: %w", err)
	}
	if err := mergeFile(cfg, globalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading global config %s: %w", globalPath, err)
	}

	// Load local config (overrides global)
	if err := mergeFile(cfg, "lfx.toml"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading local config: %w", err)
	}

	// Expand ~ in paths
	expandPaths(cfg)

	return cfg, nil
}

// LoadFrom reads a specific config file and merges it over defaults.
func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()
	if err := mergeFile(cfg, path); err != nil {
		return nil, fmt.Errorf("loading config from %s: %w", path, err)
	}
	expandPaths(cfg)
	return cfg, nil
}

// Validate checks the config for common errors.
func Validate(cfg *Config) error {
	var errs []string

	if cfg.General == nil {
		errs = append(errs, "[general] section required")
	} else {
		if strVal(cfg.General.Root) == "" {
			errs = append(errs, "[general].root cannot be empty")
		}
		if strVal(cfg.General.LibraryDir) == "" {
			errs = append(errs, "[general].library_dir cannot be empty")
		}
		if strVal(cfg.General.ManifestPath) == "" {
			errs = append(errs, "[general].manifest_path cannot be empty")
		}
		validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLogLevels[strVal(cfg.General.LogLevel)] {
			errs = append(errs, fmt.Sprintf("[general].log_level must be one of: debug, info, warn, error (got %q)", strVal(cfg.General.LogLevel)))
		}
	}

	if cfg.LLM != nil {
		validProviders := map[string]bool{"ollama": true, "groq": true, "cerebras": true}
		if !validProviders[strVal(cfg.LLM.Provider)] {
			errs = append(errs, fmt.Sprintf("[llm].provider must be one of: ollama, groq, cerebras (got %q)", strVal(cfg.LLM.Provider)))
		}
		if floatVal(cfg.LLM.ConfidenceThreshold) < 0 || floatVal(cfg.LLM.ConfidenceThreshold) > 1 {
			errs = append(errs, "[llm].confidence_threshold must be between 0 and 1")
		}
		if floatVal(cfg.LLM.ConfidenceWarning) < 0 || floatVal(cfg.LLM.ConfidenceWarning) > 1 {
			errs = append(errs, "[llm].confidence_warning must be between 0 and 1")
		}
		if floatVal(cfg.LLM.Temperature) < 0 || floatVal(cfg.LLM.Temperature) > 2 {
			errs = append(errs, "[llm].temperature must be between 0 and 2")
		}
	}

	if cfg.Apply != nil {
		validCompression := map[string]bool{"ocrmypdf": true, "ghostscript": true, "none": true}
		if !validCompression[strVal(cfg.Apply.Compression)] {
			errs = append(errs, fmt.Sprintf("[apply].compression must be one of: ocrmypdf, ghostscript, none (got %q)", strVal(cfg.Apply.Compression)))
		}
		validConflict := map[string]bool{"suffix": true, "overwrite": true, "skip": true, "ask": true}
		if !validConflict[strVal(cfg.Apply.Conflict)] {
			errs = append(errs, fmt.Sprintf("[apply].conflict must be one of: suffix, overwrite, skip, ask (got %q)", strVal(cfg.Apply.Conflict)))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// mergeFile reads a TOML file and merges its values into cfg.
// Only non-zero values from the file override the existing config.
func mergeFile(cfg *Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Decode into a temporary config — only set values that are present in the file
	var fileCfg Config
	if _, err := toml.NewDecoder(f).Decode(&fileCfg); err != nil {
		return err
	}

	merge(cfg, &fileCfg)
	return nil
}

// merge overlays non-zero values from src onto dst.
func merge(dst, src *Config) {
	if src.General != nil {
		if dst.General == nil {
			dst.General = &GeneralConfig{}
		}
		mergeGeneral(dst.General, src.General)
	}
	if src.Scan != nil {
		if dst.Scan == nil {
			dst.Scan = &ScanConfig{}
		}
		mergeScan(dst.Scan, src.Scan)
	}
	if src.LLM != nil {
		if dst.LLM == nil {
			dst.LLM = &LLMConfig{}
		}
		mergeLLM(dst.LLM, src.LLM)
	}
	if src.Library != nil {
		if dst.Library == nil {
			dst.Library = &LibraryConfig{}
		}
		mergeLibrary(dst.Library, src.Library)
	}
	if src.Metadata != nil {
		if dst.Metadata == nil {
			dst.Metadata = &MetadataConfig{}
		}
		mergeMetadata(dst.Metadata, src.Metadata)
	}
	if src.OCR != nil {
		if dst.OCR == nil {
			dst.OCR = &OCRConfig{}
		}
		mergeOCR(dst.OCR, src.OCR)
	}
	if src.Apply != nil {
		if dst.Apply == nil {
			dst.Apply = &ApplyConfig{}
		}
		mergeApply(dst.Apply, src.Apply)
	}
	if src.Kavita != nil {
		if dst.Kavita == nil {
			dst.Kavita = &KavitaConfig{}
		}
		mergeKavita(dst.Kavita, src.Kavita)
	}
}

func mergeGeneral(dst, src *GeneralConfig) {
	if src.Root != nil {
		dst.Root = src.Root
	}
	if src.LibraryDir != nil {
		dst.LibraryDir = src.LibraryDir
	}
	if src.ManifestPath != nil {
		dst.ManifestPath = src.ManifestPath
	}
	if src.LogLevel != nil {
		dst.LogLevel = src.LogLevel
	}
	if src.DryRun != nil {
		dst.DryRun = src.DryRun
	}
}

func mergeScan(dst, src *ScanConfig) {
	if src.MaxDepth != nil {
		dst.MaxDepth = src.MaxDepth
	}
	if src.MinFileSizeKB != nil {
		dst.MinFileSizeKB = src.MinFileSizeKB
	}
	if src.MaxFileSizeMB != nil {
		dst.MaxFileSizeMB = src.MaxFileSizeMB
	}
	if len(src.ExcludeDirs) > 0 {
		dst.ExcludeDirs = src.ExcludeDirs
	}
	if len(src.Extensions) > 0 {
		dst.Extensions = src.Extensions
	}
}

func mergeLLM(dst, src *LLMConfig) {
	if src.Provider != nil {
		dst.Provider = src.Provider
	}
	if src.Model != nil {
		dst.Model = src.Model
	}
	if src.FallbackModel != nil {
		dst.FallbackModel = src.FallbackModel
	}
	if src.ConfidenceThreshold != nil {
		dst.ConfidenceThreshold = src.ConfidenceThreshold
	}
	if src.ConfidenceWarning != nil {
		dst.ConfidenceWarning = src.ConfidenceWarning
	}
	if src.Temperature != nil {
		dst.Temperature = src.Temperature
	}
	if src.MaxConcurrent != nil {
		dst.MaxConcurrent = src.MaxConcurrent
	}
	if src.TimeoutSeconds != nil {
		dst.TimeoutSeconds = src.TimeoutSeconds
	}
	if len(src.LocalOnlyCategories) > 0 {
		dst.LocalOnlyCategories = src.LocalOnlyCategories
	}
	if src.Retry != nil {
		dst.Retry = src.Retry
	}
	if src.MaxRetries != nil {
		dst.MaxRetries = src.MaxRetries
	}
}

func mergeLibrary(dst, src *LibraryConfig) {
	if src.Layout != nil {
		dst.Layout = src.Layout
	}
	if src.Types != nil {
		if dst.Types == nil {
			dst.Types = &LibraryTypes{}
		}
		if len(src.Types.ValidTypes) > 0 {
			dst.Types.ValidTypes = src.Types.ValidTypes
		}
	}
	if src.Subjects != nil {
		if dst.Subjects == nil {
			dst.Subjects = &LibrarySubjects{}
		}
		if src.Subjects.Strategy != nil {
			dst.Subjects.Strategy = src.Subjects.Strategy
		}
		if src.Subjects.MergeThreshold != nil {
			dst.Subjects.MergeThreshold = src.Subjects.MergeThreshold
		}
	}
}

func mergeMetadata(dst, src *MetadataConfig) {
	if src.EbookMetaPath != nil {
		dst.EbookMetaPath = src.EbookMetaPath
	}
	if src.FetchMetadataPath != nil {
		dst.FetchMetadataPath = src.FetchMetadataPath
	}
	if len(src.MetadataSources) > 0 {
		dst.MetadataSources = src.MetadataSources
	}
}

func mergeOCR(dst, src *OCRConfig) {
	if len(src.Languages) > 0 {
		dst.Languages = src.Languages
	}
	if src.MaxConcurrent != nil {
		dst.MaxConcurrent = src.MaxConcurrent
	}
	if src.Quality != nil {
		if dst.Quality == nil {
			dst.Quality = &OCRQuality{}
		}
		if src.Quality.BlurThreshold != nil {
			dst.Quality.BlurThreshold = src.Quality.BlurThreshold
		}
		if src.Quality.SkewThreshold != nil {
			dst.Quality.SkewThreshold = src.Quality.SkewThreshold
		}
		if src.Quality.ContrastMinSpread != nil {
			dst.Quality.ContrastMinSpread = src.Quality.ContrastMinSpread
		}
	}
}

func mergeApply(dst, src *ApplyConfig) {
	if src.FilenameTemplate != nil {
		dst.FilenameTemplate = src.FilenameTemplate
	}
	if src.Compression != nil {
		dst.Compression = src.Compression
	}
	if src.Conflict != nil {
		dst.Conflict = src.Conflict
	}
	if src.BackupDir != nil {
		dst.BackupDir = src.BackupDir
	}
	if src.BackupCompress != nil {
		dst.BackupCompress = src.BackupCompress
	}
	if src.BackupMaxCount != nil {
		dst.BackupMaxCount = src.BackupMaxCount
	}
	if src.BackupMaxAgeDays != nil {
		dst.BackupMaxAgeDays = src.BackupMaxAgeDays
	}
}

func mergeKavita(dst, src *KavitaConfig) {
	if src.Enabled != nil {
		dst.Enabled = src.Enabled
	}
	if src.DockerComposePath != nil {
		dst.DockerComposePath = src.DockerComposePath
	}
	if src.Port != nil {
		dst.Port = src.Port
	}
}

// expandPaths expands ~ in all path fields to absolute paths.
func expandPaths(cfg *Config) {
	if cfg.General != nil {
		cfg.General.Root = strPtr(expandTilde(strVal(cfg.General.Root)))
		cfg.General.LibraryDir = strPtr(expandTilde(strVal(cfg.General.LibraryDir)))
		cfg.General.ManifestPath = strPtr(expandTilde(strVal(cfg.General.ManifestPath)))
	}
	if cfg.Kavita != nil {
		cfg.Kavita.DockerComposePath = strPtr(expandTilde(strVal(cfg.Kavita.DockerComposePath)))
	}
	if cfg.Apply != nil {
		cfg.Apply.BackupDir = strPtr(expandTilde(strVal(cfg.Apply.BackupDir)))
	}
}

// expandTilde replaces ~ with the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path // can't expand, return as-is
	}
	return filepath.Join(home, path[1:])
}

// globalConfigPath returns ~/.config/lfx/lfx.toml.
func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lfx", "lfx.toml"), nil
}
