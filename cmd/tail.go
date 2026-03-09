package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"logtailr/internal/config"
	"logtailr/internal/filter"
	"logtailr/internal/health"
	"logtailr/internal/output"
	"logtailr/internal/parser"
	"logtailr/internal/tailer"
	"logtailr/pkg/logline"

	"github.com/spf13/cobra"
)

const (
	maxErrorMsgLen   = 20
	maxSourceNameLen = 27
	logChannelBuffer = 100
	errChannelBuffer = 10
)

var (
	filePath    string
	level       string
	regex       string
	follow      bool
	parserFlag  string
	outputFlag  string
	outputPath  string
	showHealth  bool
	healthEvery int
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Tail and parse logs from multiple sources",
	Long: `Tail logs from files, Docker containers, and journalctl simultaneously.

Use --file for a single source or --config for multiple sources defined in YAML.`,
	RunE: runTail,
}

func init() {
	rootCmd.AddCommand(tailCmd)

	tailCmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to a single log file (shortcut for simple usage)")
	tailCmd.Flags().StringVarP(&level, "level", "l", "info", "Minimum log level (debug, info, warn, error, fatal)")
	tailCmd.Flags().StringVarP(&regex, "regex", "r", "", "Regex pattern to filter messages")
	tailCmd.Flags().BoolVar(&follow, "follow", true, "Follow sources for new lines")
	tailCmd.Flags().StringVarP(&parserFlag, "parser", "p", "", "Log format: json, logfmt, text (default: auto-detect)")
	tailCmd.Flags().StringVarP(&outputFlag, "output", "o", "console", "Output format: console, json, file")
	tailCmd.Flags().StringVar(&outputPath, "output-path", "", "Output file path (required when --output=file)")
	tailCmd.Flags().BoolVar(&showHealth, "show-health", false, "Show health status of sources")
	tailCmd.Flags().IntVar(&healthEvery, "health-every", 0, "Show health updates every N seconds (0 = disabled)")
}

func runTail(cmd *cobra.Command, _ []string) error {
	// Build source list: either from --config or from --file
	sources, err := buildSources(cmd)
	if err != nil {
		return err
	}

	// Validate global flags
	if _, ok := logline.LogLevels[strings.ToLower(level)]; !ok {
		return fmt.Errorf("invalid log level %q: must be one of debug, info, warn, error, fatal", level)
	}

	regexFilter, err := filter.NewRegexFilter(regex)
	if err != nil {
		return err
	}

	if outputFlag == "file" && outputPath == "" {
		return fmt.Errorf("--output-path is required when --output=file")
	}

	// Context with signal handling
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Initialize shared components
	healthMonitor := health.NewMonitor()
	writer, err := createWriter()
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	// Shared channels — all tailers write to the same pipeline
	logChan := make(chan *logline.LogLine, logChannelBuffer*len(sources))
	errChan := make(chan error, errChannelBuffer*len(sources))

	// Start a tailer for each source
	var tailers []tailer.Tailer
	for _, src := range sources {
		t, err := createTailer(src, healthMonitor)
		if err != nil {
			return fmt.Errorf("source %q: %w", src.Name, err)
		}
		tailers = append(tailers, t)
		t.Start(ctx, logChan, errChan)
	}

	// Print startup info
	printStartupBanner(sources)

	if showHealth {
		printHealthStatus(healthMonitor)
		startHealthUpdater(ctx, healthMonitor)
	}

	// Run the pipeline (blocks until ctx is cancelled)
	result := runPipeline(ctx, logChan, errChan, regexFilter, writer, healthMonitor)

	// Stop all tailers
	for _, t := range tailers {
		_ = t.Stop()
	}

	return result
}

func buildSources(cmd *cobra.Command) ([]logline.SourceConfig, error) {
	// If --config is set, load from YAML
	if cfgFile != "" {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return nil, err
		}
		// Apply global config overrides from flags if set
		if cfg.Global.Level != "" && !cmd.Flags().Changed("level") {
			level = cfg.Global.Level
		}
		if cfg.Global.Regex != "" && regex == "" {
			regex = cfg.Global.Regex
		}
		if cfg.Global.Output != "" && outputFlag == "console" {
			outputFlag = cfg.Global.Output
		}
		if cfg.Global.OutputPath != "" && outputPath == "" {
			outputPath = cfg.Global.OutputPath
		}
		if cfg.Global.ShowHealth {
			showHealth = true
		}
		return cfg.Sources, nil
	}

	// Otherwise, require --file for single-source mode
	if filePath == "" {
		return nil, fmt.Errorf("either --file or --config is required")
	}

	absPath, err := validateFilePath(filePath)
	if err != nil {
		return nil, err
	}

	return []logline.SourceConfig{{
		Name:   absPath,
		Type:   logline.SourceTypeFile,
		Path:   absPath,
		Follow: follow,
		Parser: parserFlag,
	}}, nil
}

func validateFilePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve file path: %w", err)
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot access file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file")
	}
	return absPath, nil
}

func createTailer(src logline.SourceConfig, monitor *health.Monitor) (tailer.Tailer, error) {
	switch src.Type {
	case logline.SourceTypeFile:
		return tailer.NewFileTailer(src.Path, src.Follow, monitor), nil
	case logline.SourceTypeDocker:
		return tailer.NewDockerTailer(src.Container, src.Follow, monitor), nil
	case logline.SourceTypeJournalctl:
		return tailer.NewJournalctlTailer(src.Unit, src.Follow, monitor), nil
	case logline.SourceTypeStdin:
		return tailer.NewStdinTailer(monitor), nil
	default:
		return nil, fmt.Errorf("unsupported source type %q", src.Type)
	}
}

func runPipeline(
	ctx context.Context,
	logChan <-chan *logline.LogLine,
	errChan <-chan error,
	regexFilter *filter.RegexFilter,
	writer output.Writer,
	healthMonitor *health.Monitor,
) error {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n" + healthMonitor.Summary())
			return nil

		case err := <-errChan:
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		case raw, ok := <-logChan:
			if !ok {
				fmt.Println("\n" + healthMonitor.Summary())
				return nil
			}

			// Parse: use a per-source parser to detect format
			logParser := parser.New(raw.Source)
			parsed, err := logParser.Parse(raw.Message, parserFlag)
			if err != nil {
				parsed = raw
			} else {
				parsed.Source = raw.Source
			}

			// Filter: level
			if !filter.ByLevel(parsed, level) {
				continue
			}

			// Filter: regex
			if !regexFilter.Match(parsed.Message) {
				continue
			}

			// Output
			if err := writer.Write(parsed); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Output error: %v\n", err)
			}
		}
	}
}

func printStartupBanner(sources []logline.SourceConfig) {
	fmt.Printf("Logtailr started | %d source(s) | level>=%s | output=%s\n", len(sources), level, outputFlag)
	for _, src := range sources {
		detail := sourceDetail(src)
		fmt.Printf("  -> [%s] %s (%s)\n", src.Type, src.Name, detail)
	}
}

func sourceDetail(src logline.SourceConfig) string {
	switch src.Type {
	case logline.SourceTypeFile:
		if src.Follow {
			return "follow"
		}
		return "read-once"
	case logline.SourceTypeDocker:
		return "container=" + src.Container
	case logline.SourceTypeJournalctl:
		return "unit=" + src.Unit
	case logline.SourceTypeStdin:
		return "pipe"
	default:
		return src.Type
	}
}

func createWriter() (output.Writer, error) {
	switch outputFlag {
	case "json":
		return output.NewJSONWriter(os.Stdout), nil
	case "file":
		return output.NewFileWriter(outputPath)
	default:
		return output.NewConsoleWriter(), nil
	}
}

func startHealthUpdater(ctx context.Context, monitor *health.Monitor) {
	if healthEvery <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Duration(healthEvery) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Println("\n--- Health Update ---")
				printHealthStatus(monitor)
			}
		}
	}()
}

func printHealthStatus(monitor *health.Monitor) {
	statuses := monitor.GetAllStatuses()

	if len(statuses) == 0 {
		fmt.Println("No sources registered")
		return
	}

	printTableHeader()
	printTableRows(statuses)
	printTableFooter()
}

func printTableHeader() {
	fmt.Println("\nSources Health:")
	fmt.Println("┌─────────────────────────────┬───────────┬──────────────┬──────────────────────┐")
	fmt.Println("│ Source                      │ Status    │ Error Count  │ Last Error           │")
	fmt.Println("├─────────────────────────────┼───────────┼──────────────┼──────────────────────┤")
}

func printTableRows(statuses map[string]*health.SourceHealth) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	defer func() { _ = w.Flush() }()

	for _, s := range statuses {
		_, _ = fmt.Fprintf(w, "│ %-27s │ %-9s │ %-12d │ %-20s │\n",
			truncate(s.Name, maxSourceNameLen),
			formatStatus(s.Status),
			s.ErrorCount,
			formatError(s.LastError),
		)
	}
}

func printTableFooter() {
	fmt.Println("└─────────────────────────────┴───────────┴──────────────┴──────────────────────┘")
}

func formatStatus(status health.Status) string {
	return fmt.Sprintf("%s %s", status.Symbol(), status)
}

func formatError(err error) string {
	if err == nil {
		return "-"
	}

	msg := err.Error()
	if len(msg) > maxErrorMsgLen {
		return msg[:maxErrorMsgLen-3] + "..."
	}
	return msg
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
