package main

import (
	"fmt"
	"os"

	"github.com/alexmatias/librarian_fox/internal/config"
	"github.com/spf13/cobra"
)

var configPath string

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Gestionar configuración",
		Long:  "Mostrar la configuración resuelta o validar el archivo de configuración.",
	}

	cmd.PersistentFlags().StringVar(&configPath, "config", "", "Ruta al archivo de configuración TOML")

	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigValidateCmd(),
	)

	return cmd
}

func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.LoadFrom(configPath)
	}
	return config.Load()
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Mostrar configuración resuelta",
		Long:  "Muestra la configuración final después de aplicar defaults, archivos TOML y flags CLI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("error cargando configuración: %w", err)
			}

			fmt.Printf("Configuración resuelta:\n\n")
			fmt.Printf("[general]\n")
			fmt.Printf("  root = %q\n", config.StrVal(cfg.General.Root))
			fmt.Printf("  library_dir = %q\n", config.StrVal(cfg.General.LibraryDir))
			fmt.Printf("  manifest_path = %q\n", config.StrVal(cfg.General.ManifestPath))
			fmt.Printf("  log_level = %q\n", config.StrVal(cfg.General.LogLevel))
			fmt.Printf("  dry_run = %v\n", config.BoolVal(cfg.General.DryRun))

			fmt.Printf("\n[scan]\n")
			fmt.Printf("  max_depth = %d\n", config.IntVal(cfg.Scan.MaxDepth))
			fmt.Printf("  min_file_size_kb = %d\n", config.IntVal(cfg.Scan.MinFileSizeKB))
			fmt.Printf("  max_file_size_mb = %d\n", config.IntVal(cfg.Scan.MaxFileSizeMB))
			fmt.Printf("  extensions = %v\n", cfg.Scan.Extensions)

			fmt.Printf("\n[llm]\n")
			fmt.Printf("  provider = %q\n", config.StrVal(cfg.LLM.Provider))
			fmt.Printf("  model = %q\n", config.StrVal(cfg.LLM.Model))
			fmt.Printf("  fallback_model = %q\n", config.StrVal(cfg.LLM.FallbackModel))
			fmt.Printf("  confidence_threshold = %.2f\n", config.FloatVal(cfg.LLM.ConfidenceThreshold))
			fmt.Printf("  confidence_warning = %.2f\n", config.FloatVal(cfg.LLM.ConfidenceWarning))
			fmt.Printf("  temperature = %.2f\n", config.FloatVal(cfg.LLM.Temperature))
			fmt.Printf("  max_concurrent = %d\n", config.IntVal(cfg.LLM.MaxConcurrent))
			fmt.Printf("  timeout_seconds = %d\n", config.IntVal(cfg.LLM.TimeoutSeconds))
			fmt.Printf("  retry = %v\n", config.BoolVal(cfg.LLM.Retry))
			fmt.Printf("  max_retries = %d\n", config.IntVal(cfg.LLM.MaxRetries))

			fmt.Printf("\n[library]\n")
			fmt.Printf("  layout = %q\n", config.StrVal(cfg.Library.Layout))

			if cfg.Library.Types != nil && len(cfg.Library.Types.ValidTypes) > 0 {
				fmt.Printf("\n[library.types]\n")
				fmt.Printf("  valid_types = %v\n", cfg.Library.Types.ValidTypes)
			}

			if cfg.Library.Subjects != nil {
				fmt.Printf("\n[library.subjects]\n")
				fmt.Printf("  strategy = %q\n", config.StrVal(cfg.Library.Subjects.Strategy))
				fmt.Printf("  merge_threshold = %.2f\n", config.FloatVal(cfg.Library.Subjects.MergeThreshold))
			}

			fmt.Printf("\n[metadata]\n")
			fmt.Printf("  ebook_meta_path = %q\n", config.StrVal(cfg.Metadata.EbookMetaPath))
			fmt.Printf("  fetch_metadata_path = %q\n", config.StrVal(cfg.Metadata.FetchMetadataPath))
			fmt.Printf("  metadata_sources = %v\n", cfg.Metadata.MetadataSources)

			fmt.Printf("\n[ocr]\n")
			fmt.Printf("  languages = %v\n", cfg.OCR.Languages)
			fmt.Printf("  max_concurrent = %d\n", config.IntVal(cfg.OCR.MaxConcurrent))
			if cfg.OCR.Quality != nil {
				fmt.Printf("\n[ocr.quality]\n")
				fmt.Printf("  blur_threshold = %.1f\n", config.FloatVal(cfg.OCR.Quality.BlurThreshold))
				fmt.Printf("  skew_threshold = %.1f\n", config.FloatVal(cfg.OCR.Quality.SkewThreshold))
				fmt.Printf("  contrast_min_spread = %.1f\n", config.FloatVal(cfg.OCR.Quality.ContrastMinSpread))
			}

			fmt.Printf("\n[apply]\n")
			fmt.Printf("  filename_template = %q\n", config.StrVal(cfg.Apply.FilenameTemplate))
			fmt.Printf("  compression = %q\n", config.StrVal(cfg.Apply.Compression))
			fmt.Printf("  conflict = %q\n", config.StrVal(cfg.Apply.Conflict))
			fmt.Printf("  backup_dir = %q\n", config.StrVal(cfg.Apply.BackupDir))
			fmt.Printf("  backup_compress = %v\n", config.BoolVal(cfg.Apply.BackupCompress))
			fmt.Printf("  backup_max_count = %d\n", config.IntVal(cfg.Apply.BackupMaxCount))
			fmt.Printf("  backup_max_age_days = %d\n", config.IntVal(cfg.Apply.BackupMaxAgeDays))

			if cfg.Kavita.Enabled != nil && *cfg.Kavita.Enabled {
				fmt.Printf("\n[kavita]\n")
				fmt.Printf("  enabled = true\n")
				fmt.Printf("  port = %d\n", config.IntVal(cfg.Kavita.Port))
			}

			return nil
		},
	}
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validar archivo de configuración",
		Long:  "Valida el archivo de configuración TOML y muestra errores si los hay.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := loadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error de validación:\n%v\n", err)
				os.Exit(1)
			}
			fmt.Println("Configuración válida ✓")
			return nil
		},
	}
}
