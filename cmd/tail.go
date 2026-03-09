package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"logtailr/internal/api"
	"logtailr/internal/config"
	"logtailr/internal/filter"
	"logtailr/internal/health"
	"logtailr/internal/tailer"
	"logtailr/pkg/logline"

	"github.com/spf13/cobra"
)

const (
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
