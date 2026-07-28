package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexmatias/librarian_fox/internal/config"
	"github.com/alexmatias/librarian_fox/internal/manifest"
	"github.com/alexmatias/librarian_fox/internal/scan"
	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [root]",
		Short: "Discover documents and populate manifest",
		Long:  "Scan a directory tree for reading documents (PDF, DjVu, DOCX, etc.) and populate the SQLite manifest.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			dbPath, _ := cmd.Flags().GetString("db")
			logLevel, _ := cmd.Flags().GetString("log-level")
			cfgPath, _ := cmd.Flags().GetString("config")
			jsonOut, _ := cmd.Flags().GetBool("json")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			statsOnly, _ := cmd.Flags().GetBool("stats-only")
			excludeDirs, _ := cmd.Flags().GetStringSlice("exclude-dir")
			noDefaultExcludes, _ := cmd.Flags().GetBool("no-default-excludes")
			maxDepth, _ := cmd.Flags().GetInt("max-depth")

			// Positional arg overrides --root flag
			if len(args) > 0 {
				root = args[0]
			}

			if cfgPath != "" {
				configPath = cfgPath
			}

			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if root != "" {
				cfg.General.Root = config.StrPtr(root)
			}

			if dbPath != "" {
				cfg.General.ManifestPath = config.StrPtr(dbPath)
			}

			if dryRun {
				cfg.General.DryRun = config.BoolPtr(true)
			}

			// --exclude-dir is additive: appends to config defaults.
			// Use --no-default-excludes to start from empty list.
			if len(excludeDirs) > 0 {
				if noDefaultExcludes {
					cfg.Scan.ExcludeDirs = excludeDirs
				} else {
					cfg.Scan.ExcludeDirs = append(cfg.Scan.ExcludeDirs, excludeDirs...)
				}
			}

			if maxDepth > 0 {
				cfg.Scan.MaxDepth = config.IntPtr(maxDepth)
			}

			if logLevel == "" {
				logLevel = config.StrVal(cfg.General.LogLevel)
			}
			level := slog.LevelInfo
			switch logLevel {
			case "debug":
				level = slog.LevelDebug
			case "warn":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}))

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			var db *manifest.DB
			if !statsOnly {
				db, err = manifest.Open(config.StrVal(cfg.General.ManifestPath))
				if err != nil {
					return fmt.Errorf("opening manifest: %w", err)
				}
				defer func() { _ = db.Close() }()

				if err := db.Migrate(ctx); err != nil {
					return fmt.Errorf("migrating manifest: %w", err)
				}
			}

			scanner := scan.New(cfg, db, logger)
			scanner.DryRun = dryRun
			scanner.StatsOnly = statsOnly

			// Progress display goroutine: collect phase + scan bar [████░░░░]
			var progress chan *scan.Event
			var progressDone chan struct{}
			if !jsonOut && !statsOnly {
				progress = make(chan *scan.Event, 100)
				progressDone = make(chan struct{})
				go func() {
					defer close(progressDone)
					for e := range progress {
						if e.Phase == "collect" {
							fmt.Fprintf(os.Stderr, "\rCollecting... %d files found...", e.Current)
							continue
						}
						bar := drawProgressBar(e.Current, e.Total)
						if e.Current >= e.Total {
							fmt.Fprintf(os.Stderr, "\rScanning... %s %d/%d files\n", bar, e.Current, e.Total)
						} else {
							fmt.Fprintf(os.Stderr, "\rScanning... %s %d/%d files", bar, e.Current, e.Total)
						}
					}
					fmt.Fprintln(os.Stderr)
				}()
			}

			result, err := scanner.Scan(ctx, config.StrVal(cfg.General.Root), progress)

			// Stop progress goroutine before printing output
			if progress != nil {
				close(progress)
				<-progressDone
			}

			// Handle context cancellation (Ctrl+C)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					if !jsonOut {
						fmt.Printf("Scan interrupted after %d files, %d already in DB\n",
							result.Stats.TotalFiles, result.Stats.TotalFiles)
					} else {
						printJSONError("interrupted", result.Stats)
					}
					return nil
				}
				return fmt.Errorf("scanning: %w", err)
			}

			if jsonOut {
				printJSONResult(result)
				return nil
			}

			if dryRun {
				fmt.Print("Dry run — nothing was written to the database.\n\n")
			}

			if statsOnly {
				fmt.Printf("Stats (Phase 1 only):\n")
			} else {
				fmt.Printf("Scan complete:\n")
			}
			fmt.Printf("  Documents found: %d\n", result.Stats.TotalFiles)
			fmt.Printf("  Total size: %.1f MB\n", float64(result.Stats.TotalSize)/1048576)
			fmt.Printf("  Time: %.1fs (collect: %dms, process: %dms)\n",
				result.Stats.Duration.Seconds(),
				result.Stats.CollectMs,
				result.Stats.ProcessMs)
			fmt.Printf("  By type:\n")
			for t, n := range result.Stats.ByType {
				fmt.Printf("    %s: %d\n", t, n)
			}
			if !statsOnly {
				if result.Stats.WithTextLayer > 0 {
					fmt.Printf("  With text layer: %d\n", result.Stats.WithTextLayer)
				}
				if result.Stats.NeedingOCR > 0 {
					fmt.Printf("  Needs OCR: %d\n", result.Stats.NeedingOCR)
				}
			}
			if result.Stats.SkippedOnMtime > 0 {
				fmt.Printf("  Unchanged (skipped): %d\n", result.Stats.SkippedOnMtime)
			}
			if result.Stats.SkippedBySize > 0 {
				fmt.Printf("  Skipped by size: %d\n", result.Stats.SkippedBySize)
			}
			if result.Stats.SkippedByDir > 0 {
				fmt.Printf("  Skipped directories: %d\n", result.Stats.SkippedByDir)
			}
			if result.Stats.SymlinksSkipped > 0 {
				fmt.Printf("  Symlinks skipped: %d\n", result.Stats.SymlinksSkipped)
			}
			if len(result.Errors) > 0 {
				fmt.Printf("  Errors: %d\n", len(result.Errors))
				for _, e := range result.Errors {
					fmt.Printf("    %s: %v\n", e.Path, e.Error)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("root", "", "Root directory to scan (overrides config and positional arg)")
	cmd.Flags().String("db", "", "Path to SQLite manifest database (overrides config)")
	cmd.Flags().String("config", "", "Path to TOML config file")
	cmd.Flags().String("log-level", "", "Log level (debug, info, warn, error) — defaults to config value")
	cmd.Flags().Bool("json", false, "Output results as JSON")
	cmd.Flags().Bool("dry-run", false, "Process files fully but skip DB writes — report only")
	cmd.Flags().Bool("stats-only", false, "Quick file count by extension/size — no hashing, no subprocess, no DB")
	cmd.Flags().StringSlice("exclude-dir", nil, "Additional directory names to exclude (repeatable, additive)")
	cmd.Flags().Bool("no-default-excludes", false, "Start with empty exclude list instead of config defaults")
	cmd.Flags().Int("max-depth", 0, "Maximum directory traversal depth (0 = use config default)")

	return cmd
}

func printJSONResult(result *scan.Result) {
	out := map[string]interface{}{
		"status": "completed",
		"stats": map[string]interface{}{
			"total_files":      result.Stats.TotalFiles,
			"total_size_bytes": result.Stats.TotalSize,
			"total_size_mb":    float64(result.Stats.TotalSize) / 1048576,
			"duration_ms":      result.Stats.Duration.Milliseconds(),
			"by_type":          result.Stats.ByType,
			"with_text_layer":  result.Stats.WithTextLayer,
			"needs_ocr":        result.Stats.NeedingOCR,
			"skipped_by_size":  result.Stats.SkippedBySize,
			"skipped_dirs":     result.Stats.SkippedByDir,
			"symlinks_skipped": result.Stats.SymlinksSkipped,
			"skipped_on_mtime": result.Stats.SkippedOnMtime,
		},
	}
	if len(result.Errors) > 0 {
		errs := make([]map[string]string, len(result.Errors))
		for i, e := range result.Errors {
			errs[i] = map[string]string{"path": e.Path, "error": e.Error.Error()}
		}
		out["errors"] = errs
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func drawProgressBar(current, total int) string {
	if total <= 0 {
		return ""
	}
	const width = 10
	filled := current * width / total
	if filled > width {
		filled = width
	}
	bar := ""
	for range filled {
		bar += "█"
	}
	for range width - filled {
		bar += "░"
	}
	return bar
}

func printJSONError(status string, stats scan.Stats) {
	out := map[string]interface{}{
		"status": status,
		"stats": map[string]interface{}{
			"total_files":      stats.TotalFiles,
			"total_size_bytes": stats.TotalSize,
			"total_size_mb":    float64(stats.TotalSize) / 1048576,
			"duration_ms":      stats.Duration.Milliseconds(),
			"by_type":          stats.ByType,
			"with_text_layer":  stats.WithTextLayer,
			"needs_ocr":        stats.NeedingOCR,
			"skipped_by_size":  stats.SkippedBySize,
			"skipped_dirs":     stats.SkippedByDir,
			"symlinks_skipped": stats.SymlinksSkipped,
			"skipped_on_mtime": stats.SkippedOnMtime,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
