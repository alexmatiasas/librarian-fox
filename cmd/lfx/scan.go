package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

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
			extensions, _ := cmd.Flags().GetStringSlice("extension")
			noDefaultExtensions, _ := cmd.Flags().GetBool("no-default-extensions")
			excludeGlobs, _ := cmd.Flags().GetStringSlice("exclude-glob")
			verbose, _ := cmd.Flags().GetBool("verbose")
			format, _ := cmd.Flags().GetString("format")
			noProgress, _ := cmd.Flags().GetBool("no-progress")
			quiet, _ := cmd.Flags().GetBool("quiet")
			var minSize, maxSize byteSize
			if v, err := cmd.Flags().GetString("min-size"); err == nil && v != "" {
				if err := minSize.Set(v); err != nil {
					return fmt.Errorf("--min-size: %w", err)
				}
			}
			if v, err := cmd.Flags().GetString("max-size"); err == nil && v != "" {
				if err := maxSize.Set(v); err != nil {
					return fmt.Errorf("--max-size: %w", err)
				}
			}

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

			if len(extensions) > 0 {
				if noDefaultExtensions {
					cfg.Scan.Extensions = extensions
				} else {
					cfg.Scan.Extensions = append(cfg.Scan.Extensions, extensions...)
				}
			}

			if minSize > 0 {
				minKB := int(minSize / KB)
				if minKB == 0 {
					minKB = 1
				}
				cfg.Scan.MinFileSizeKB = config.IntPtr(minKB)
			}
			if maxSize > 0 {
				maxMB := int(maxSize / MB)
				if maxMB == 0 {
					maxMB = 1
				}
				cfg.Scan.MaxFileSizeMB = config.IntPtr(maxMB)
			}

			if len(excludeGlobs) > 0 {
				cfg.Scan.ExcludeGlobs = append(cfg.Scan.ExcludeGlobs, excludeGlobs...)
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

			if format == "" {
				format = config.StrVal(cfg.General.OutputFormat)
			}

			showProgress := !jsonOut && !statsOnly && !noProgress && !quiet

			scanner := scan.New(cfg, db, logger)
			scanner.DryRun = dryRun
			scanner.StatsOnly = statsOnly

			var progress chan *scan.Event
			var progressDone chan struct{}
			if showProgress {
				progress = make(chan *scan.Event, 100)
				progressDone = make(chan struct{})
				go func() {
					defer close(progressDone)
					for e := range progress {
						if e.Phase == "collect" {
							if e.Total > 0 {
								fmt.Fprintf(os.Stderr, "\rCollecting... %d files found...\n", e.Current)
							} else {
								fmt.Fprintf(os.Stderr, "\rCollecting... %d files found...", e.Current)
							}
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

			if progress != nil {
				close(progress)
				<-progressDone
			}

			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					if jsonOut {
						printJSONError("interrupted", result.Stats)
					} else {
						fmt.Printf("Scan interrupted after %d files, %d already in DB\n",
							result.Stats.TotalFiles, result.Stats.TotalFiles)
					}
					return nil
				}
				return fmt.Errorf("scanning: %w", err)
			}

			if jsonOut {
				printJSONResult(result)
				return nil
			}

			if quiet && len(result.Errors) == 0 {
				return nil
			}

			out := newScanOutput(os.Stdout, format, verbose)

			if dryRun {
				insertCount := len(result.Documents)
				skipCount := result.Stats.SkippedOnMtime
				if insertCount > 0 && skipCount > 0 {
					out.line(fmt.Sprintf("Dry run — would insert/update %d files, %d unchanged (no DB writes)",
						insertCount, skipCount))
				} else if insertCount > 0 {
					out.line(fmt.Sprintf("Dry run — would insert/update %d files (no DB writes)", insertCount))
				} else if skipCount > 0 {
					out.line(fmt.Sprintf("Dry run — all %d files unchanged (no DB writes)", skipCount))
				} else {
					out.line("Dry run — nothing to do (no files found)")
				}
				out.blank()
			}

			if statsOnly {
				out.detail("Stats", "Phase 1 only — no hashing or subprocess")
			}

			out.summary(result.Stats)
			out.byType(result.Stats.ByType)

			if !statsOnly && (result.Stats.WithTextLayer > 0 || result.Stats.NeedingOCR > 0) {
				out.blank()
				out.header("Text layer")
				if result.Stats.WithTextLayer > 0 {
					out.detail("With text", fmt.Sprintf("%d", result.Stats.WithTextLayer))
				}
				if result.Stats.NeedingOCR > 0 {
					out.detail("Needs OCR", fmt.Sprintf("%d", result.Stats.NeedingOCR))
				}
			}

			var skippedPairs []kv
			if result.Stats.SkippedOnMtime > 0 {
				skippedPairs = append(skippedPairs, kv{"Skipped (unchanged)", fmt.Sprintf("%d", result.Stats.SkippedOnMtime)})
			}
			if result.Stats.SkippedBySize > 0 {
				skippedPairs = append(skippedPairs, kv{"Skipped by size", fmt.Sprintf("%d", result.Stats.SkippedBySize)})
			}
			if result.Stats.SkippedByDir > 0 {
				skippedPairs = append(skippedPairs, kv{"Skipped directories", fmt.Sprintf("%d", result.Stats.SkippedByDir)})
			}
			if result.Stats.SkippedByGlob > 0 {
				skippedPairs = append(skippedPairs, kv{"Skipped by glob", fmt.Sprintf("%d", result.Stats.SkippedByGlob)})
			}
			if result.Stats.SymlinksSkipped > 0 {
				skippedPairs = append(skippedPairs, kv{"Symlinks skipped", fmt.Sprintf("%d", result.Stats.SymlinksSkipped)})
			}
			if len(skippedPairs) > 0 {
				out.blank()
				out.header("Skipped")
				for _, p := range skippedPairs {
					out.detail(p.label, p.value)
				}
			}

			if len(result.Errors) > 0 {
				out.blank()
				out.header("Errors")
				for _, e := range result.Errors {
					out.detail(e.Path, e.Error.Error())
				}
			}

			if verbose && len(result.Documents) > 0 {
				out.blank()
				out.files(result.Documents)
			}

			out.flush()

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
	cmd.Flags().StringSlice("extension", nil, "File extensions to scan (repeatable, additive, without dot)")
	cmd.Flags().Bool("no-default-extensions", false, "Scan only extensions passed via --extension (ignore config defaults)")
	cmd.Flags().String("min-size", "", "Minimum file size with suffix (e.g. 10k, 1m). Suffix: k=KB, m=MB")
	cmd.Flags().String("max-size", "", "Maximum file size with suffix (e.g. 500m, 1g). Suffix: m=MB, g=GB")
	cmd.Flags().StringSlice("exclude-glob", nil, "Exclude files matching glob patterns (repeatable, e.g. *.tmp *.bak)")
	cmd.Flags().BoolP("verbose", "v", false, "List each processed file with details")
	cmd.Flags().String("format", "", fmt.Sprintf("Output format: text or table (default: %q)", config.StrVal(config.DefaultConfig().General.OutputFormat)))
	cmd.Flags().Bool("no-progress", false, "Suppress progress bar (still shows results)")
	cmd.Flags().BoolP("quiet", "q", false, "Suppress progress and output (shows errors only)")

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
			"skipped_by_glob":  result.Stats.SkippedByGlob,
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
			"skipped_by_glob":  stats.SkippedByGlob,
			"symlinks_skipped": stats.SymlinksSkipped,
			"skipped_on_mtime": stats.SkippedOnMtime,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// byteSize parses size strings with optional suffix: k/K (KB), m/M (MB), g/G (GB), t/T (TB).
// Default suffix (no letter) = bytes.
type byteSize int64

const (
	_  = iota
	KB = 1 << (10 * iota) // 1024
	MB                    // 1048576
	GB                    // 1073741824
	TB                    // 1099511627776
)

func (b *byteSize) String() string {
	switch {
	case *b >= TB:
		return fmt.Sprintf("%.0ft", float64(*b)/TB)
	case *b >= GB:
		return fmt.Sprintf("%.0fg", float64(*b)/GB)
	case *b >= MB:
		return fmt.Sprintf("%.0fm", float64(*b)/MB)
	case *b >= KB:
		return fmt.Sprintf("%.0fk", float64(*b)/KB)
	default:
		return fmt.Sprintf("%db", *b)
	}
}

func (b *byteSize) Set(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("empty size")
	}

	var multiplier int64 = 1
	numStr := s
	switch last := s[len(s)-1]; last {
	case 't', 'T':
		multiplier = TB
		numStr = s[:len(s)-1]
	case 'g', 'G':
		multiplier = GB
		numStr = s[:len(s)-1]
	case 'm', 'M':
		multiplier = MB
		numStr = s[:len(s)-1]
	case 'k', 'K':
		multiplier = KB
		numStr = s[:len(s)-1]
	}

	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid size %q: %w", s, err)
	}
	if n < 0 {
		return fmt.Errorf("negative size %q", s)
	}

	*b = byteSize(n * multiplier)
	return nil
}

func (b *byteSize) Type() string { return "size" }

// ---------------------------------------------------------------------------
// scanOutput — formatea resultados de scan en text o table mode
// ---------------------------------------------------------------------------

type kv struct{ label, value string }

type scanOutput struct {
	w       *tabwriter.Writer
	mode    string // "text" or "table"
	verbose bool
}

func newScanOutput(dst io.Writer, format string, verbose bool) *scanOutput {
	mode := format
	if mode == "" {
		mode = "table"
	}
	w := tabwriter.NewWriter(dst, 0, 0, 3, ' ', 0)
	return &scanOutput{w: w, mode: mode, verbose: verbose}
}

func (o *scanOutput) flush() { _ = o.w.Flush() }

func (o *scanOutput) blank() {
	_, _ = fmt.Fprintln(o.w)
}

func (o *scanOutput) header(title string) {
	if o.mode == "table" {
		_, _ = fmt.Fprintf(o.w, "─── %s ───\n", title)
	} else {
		_, _ = fmt.Fprintf(o.w, "%s:\n", title)
	}
}

func (o *scanOutput) detail(label, value string) {
	if o.mode == "table" {
		_, _ = fmt.Fprintf(o.w, "%s\t%s\n", label, value)
	} else {
		_, _ = fmt.Fprintf(o.w, "  %s: %s\n", label, value)
	}
}

func (o *scanOutput) line(msg string) {
	_, _ = fmt.Fprintln(o.w, msg)
}

func (o *scanOutput) summary(s scan.Stats) {
	if o.mode == "table" {
		_, _ = fmt.Fprint(o.w, "Metric\tValue\n")
		_, _ = fmt.Fprint(o.w, "───\t─────\n")
		o.detail("Documents found", fmt.Sprintf("%d", s.TotalFiles))
		o.detail("Total size", fmt.Sprintf("%.1f MB", float64(s.TotalSize)/1048576))
		o.detail("Time", fmt.Sprintf("%.1fs (collect: %dms, process: %dms)", s.Duration.Seconds(), s.CollectMs, s.ProcessMs))
	} else {
		_, _ = fmt.Fprint(o.w, "Scan complete:\n")
		o.detail("Documents found", fmt.Sprintf("%d", s.TotalFiles))
		o.detail("Total size", fmt.Sprintf("%.1f MB", float64(s.TotalSize)/1048576))
		o.detail("Time", fmt.Sprintf("%.1fs (collect: %dms, process: %dms)", s.Duration.Seconds(), s.CollectMs, s.ProcessMs))
	}
}

func (o *scanOutput) byType(byType map[string]int) {
	if len(byType) == 0 {
		return
	}
	o.blank()
	if o.mode == "table" {
		_, _ = fmt.Fprint(o.w, "Type\tCount\n")
		_, _ = fmt.Fprint(o.w, "───\t─────\n")
		for _, t := range sortedKeys(byType) {
			_, _ = fmt.Fprintf(o.w, "%s\t%d\n", t, byType[t])
		}
	} else {
		_, _ = fmt.Fprint(o.w, "  By type:\n")
		for _, t := range sortedKeys(byType) {
			_, _ = fmt.Fprintf(o.w, "    %s: %d\n", t, byType[t])
		}
	}
}

func (o *scanOutput) files(docs []manifest.Document) {
	if o.mode == "table" {
		_, _ = fmt.Fprint(o.w, "Path\tType\tPages\tText\tSize\n")
		_, _ = fmt.Fprint(o.w, "───\t───\t───\t───\t───\n")
		for _, d := range docs {
			text := "no"
			if d.HasTextLayer {
				text = "yes"
			}
			pages := "-"
			if d.PageCount != nil {
				pages = fmt.Sprintf("%d", *d.PageCount)
			}
			_, _ = fmt.Fprintf(o.w, "%s\t%s\t%s\t%s\t%s\n",
				d.SourcePath, d.FileType, pages, text, fmtSize(d.FileSize))
		}
	} else {
		for _, d := range docs {
			text := "no"
			if d.HasTextLayer {
				text = "yes"
			}
			pages := "-"
			if d.PageCount != nil {
				pages = fmt.Sprintf("%d", *d.PageCount)
			}
			_, _ = fmt.Fprintf(o.w, "  %s\n", d.SourcePath)
			_, _ = fmt.Fprintf(o.w, "    Type: %s | Pages: %s | Text: %s | Size: %s\n",
				d.FileType, pages, text, fmtSize(d.FileSize))
		}
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fmtSize(n int64) string {
	switch {
	case n >= GB:
		return fmt.Sprintf("%.1f GB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.1f MB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.0f KB", float64(n)/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
