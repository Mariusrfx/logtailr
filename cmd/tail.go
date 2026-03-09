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

	"logtailr/internal/api"
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
	maxChannelSize   = 10000
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
	apiEnabled  bool
	apiPort     int
	apiAddr     string
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
	tailCmd.Flags().BoolVar(&apiEnabled, "api", false, "Enable REST API and WebSocket server")
	tailCmd.Flags().IntVar(&apiPort, "api-port", 8080, "API server port (1024-65535)")
	tailCmd.Flags().StringVar(&apiAddr, "api-addr", "127.0.0.1", "API server bind address")
}

func runTail(cmd *cobra.Command, _ []string) error {
	// Build source list: either from --config or from --file
	sources, fullCfg, outputsCfg, err := buildSources(cmd)
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
	writer, err := createWriter(outputsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	// Start API server if enabled
	var apiServer *api.Server
	if apiEnabled {
		if apiPort < 1024 || apiPort > 65535 {
			return fmt.Errorf("--api-port must be between 1024 and 65535, got %d", apiPort)
		}
		listenAddr := fmt.Sprintf("%s:%d", apiAddr, apiPort)
		apiServer = api.NewServer(api.ServerConfig{
			Addr:    listenAddr,
			Monitor: healthMonitor,
			Config:  fullCfg,
		})
		apiServer.Start()
		defer func() { _ = apiServer.Stop() }()
	}

	// Shared channels — all tailers write to the same pipeline
	logBufSize := min(logChannelBuffer*len(sources), maxChannelSize)
	errBufSize := min(errChannelBuffer*len(sources), maxChannelSize)
	logChan := make(chan *logline.LogLine, logBufSize)
	errChan := make(chan error, errBufSize)

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
	result := runPipeline(ctx, logChan, errChan, regexFilter, writer, healthMonitor, apiServer)

	// Stop all tailers
	for _, t := range tailers {
		_ = t.Stop()
	}

	return result
}

func buildSources(cmd *cobra.Command) ([]logline.SourceConfig, *config.Config, *config.OutputsConfig, error) {
	// If --config is set, load from YAML
	if cfgFile != "" {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return nil, nil, nil, err
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
		return cfg.Sources, cfg, &cfg.Outputs, nil
	}

	// Otherwise, require --file for single-source mode
	if filePath == "" {
		return nil, nil, nil, fmt.Errorf("either --file or --config is required")
	}

	absPath, err := validateFilePath(filePath)
	if err != nil {
		return nil, nil, nil, err
	}

	return []logline.SourceConfig{{
		Name:   absPath,
		Type:   logline.SourceTypeFile,
		Path:   absPath,
		Follow: follow,
		Parser: parserFlag,
	}}, nil, nil, nil
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
		return tailer.NewDockerTailer(src.Container, src.Follow, monitor)
	case logline.SourceTypeJournalctl:
		return tailer.NewJournalctlTailer(src.Unit, src.Follow, monitor)
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
	apiServer *api.Server,
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

			// Record metrics before filtering (total processed)
			if apiServer != nil {
				safeSource := api.SanitizeLabel(parsed.Source, 128)
				safeLevel := api.SanitizeLabel(parsed.Level, 16)
				apiServer.Metrics().LogsTotal.WithLabelValues(safeSource, safeLevel).Inc()
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

			// Broadcast to WebSocket clients (after filtering)
			if apiServer != nil {
				apiServer.Hub().Broadcast(parsed)
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

func createWriter(outputsCfg *config.OutputsConfig) (output.Writer, error) {
	var primary output.Writer
	switch outputFlag {
	case "json":
		primary = output.NewJSONWriter(os.Stdout)
	case "file":
		fw, err := output.NewFileWriter(outputPath)
		if err != nil {
			return nil, err
		}
		primary = fw
	default:
		primary = output.NewConsoleWriter()
	}

	if outputsCfg == nil {
		return primary, nil
	}

	// Collect additional writers from outputs config
	writers := []output.Writer{primary}

	if outputsCfg.OpenSearch != nil && outputsCfg.OpenSearch.Enabled {
		osCfg := outputsCfg.OpenSearch
		ow, err := output.NewOpenSearchWriter(output.OpenSearchConfig{
			Hosts:         osCfg.Hosts,
			Index:         osCfg.Index,
			Username:      osCfg.Username,
			Password:      osCfg.Password,
			BulkSize:      osCfg.BulkSize,
			FlushInterval: osCfg.FlushInterval,
			TLSSkipVerify: osCfg.TLSSkipVerify,
			MaxRetries:    osCfg.MaxRetries,
		})
		if err != nil {
			return nil, fmt.Errorf("opensearch output: %w", err)
		}
		writers = append(writers, ow)
	}

	if outputsCfg.Webhook != nil && outputsCfg.Webhook.Enabled {
		wh := outputsCfg.Webhook
		ww, err := output.NewWebhookWriter(output.WebhookConfig{
			URL:          wh.URL,
			MinLevel:     wh.MinLevel,
			BatchSize:    wh.BatchSize,
			BatchTimeout: wh.BatchTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("webhook output: %w", err)
		}
		writers = append(writers, ww)
	}

	if len(writers) == 1 {
		return primary, nil
	}
	return output.NewMultiWriter(writers...), nil
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
