package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"logtailr/internal/aggregator"
	"logtailr/internal/alert"
	"logtailr/internal/api"
	"logtailr/internal/bookmark"
	"logtailr/internal/config"
	"logtailr/internal/filter"
	"logtailr/internal/health"
	"logtailr/internal/tailer"
	"logtailr/pkg/logline"
	"time"

	"github.com/spf13/cobra"
)

const (
	logChannelBuffer = 100
	errChannelBuffer = 10
	maxChannelSize   = 10000
)

var (
	filePath        string
	level           string
	regex           string
	follow          bool
	parserFlag      string
	outputFlag      string
	outputPath      string
	showHealth      bool
	healthEvery     int
	apiEnabled      bool
	apiPort         int
	apiAddr         string
	allowLocal      bool
	aggregate       bool
	aggregateWindow string
	bookmarkName    string
	resumeName      string
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
	tailCmd.Flags().BoolVar(&allowLocal, "allow-local", false, "Allow localhost/private IPs in URLs (disables SSRF prevention for local development)")
	tailCmd.Flags().BoolVar(&aggregate, "aggregate", false, "Aggregate repeated log messages")
	tailCmd.Flags().StringVar(&aggregateWindow, "aggregate-window", "5s", "Time window for aggregation (e.g. 3s, 10s)")
	tailCmd.Flags().StringVar(&bookmarkName, "bookmark", "", "Save file position with this bookmark name on exit")
	tailCmd.Flags().StringVar(&resumeName, "resume", "", "Resume from a saved bookmark position")
}

func runTail(cmd *cobra.Command, _ []string) error {
	if resumeName != "" && cfgFile != "" {
		return fmt.Errorf("--resume only works with --file, not --config")
	}
	if bookmarkName != "" {
		if err := bookmark.ValidateName(bookmarkName); err != nil {
			return fmt.Errorf("--bookmark: %w", err)
		}
	}
	if resumeName != "" {
		if err := bookmark.ValidateName(resumeName); err != nil {
			return fmt.Errorf("--resume: %w", err)
		}
	}

	sources, fullCfg, outputsCfg, err := buildSources(cmd)
	if err != nil {
		return err
	}

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

	var startOffset int64
	if resumeName != "" {
		mgr, err := bookmark.NewManager()
		if err != nil {
			return fmt.Errorf("bookmark: %w", err)
		}
		bm, err := mgr.Load(resumeName)
		if err != nil {
			return fmt.Errorf("bookmark %q: %w", resumeName, err)
		}
		if len(sources) == 1 && sources[0].Type == logline.SourceTypeFile {
			if bm.File != sources[0].Path {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: bookmark file %q differs from --file %q, reading from start\n", bm.File, sources[0].Path)
			} else {
				inode, err := bookmark.GetInode(sources[0].Path)
				if err == nil && inode != bm.Inode {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: file inode changed (was %d, now %d), reading from start\n", bm.Inode, inode)
				} else if err == nil {
					startOffset = bm.Offset
					_, _ = fmt.Fprintf(os.Stderr, "Resuming from bookmark %q at offset %d\n", resumeName, startOffset)
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	healthMonitor := health.NewMonitor()
	writer, err := createWriter(outputsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	var alertEngine *alert.Engine
	if fullCfg != nil && fullCfg.Alerts != nil && fullCfg.Alerts.Enabled {
		alertEngine, err = buildAlertEngine(fullCfg.Alerts, healthMonitor)
		if err != nil {
			return fmt.Errorf("alerts: %w", err)
		}
		defer func() { _ = alertEngine.Close() }()
	}

	var apiServer *api.Server
	if apiEnabled {
		if apiPort < 1024 || apiPort > 65535 {
			return fmt.Errorf("--api-port must be between 1024 and 65535, got %d", apiPort)
		}
		listenAddr := fmt.Sprintf("%s:%d", apiAddr, apiPort)
		apiServer = api.NewServer(api.ServerConfig{
			Addr:        listenAddr,
			Monitor:     healthMonitor,
			Config:      fullCfg,
			AlertEngine: alertEngine,
		})
		apiServer.Start()
		defer func() { _ = apiServer.Stop() }()
	}

	logBufSize := min(logChannelBuffer*len(sources), maxChannelSize)
	errBufSize := min(errChannelBuffer*len(sources), maxChannelSize)
	logChan := make(chan *logline.LogLine, logBufSize)
	errChan := make(chan error, errBufSize)

	var fileTailerRef *tailer.FileTailer
	var tailers []tailer.Tailer
	for _, src := range sources {
		t, err := createTailer(src, healthMonitor)
		if err != nil {
			return fmt.Errorf("source %q: %w", src.Name, err)
		}
		if ft, ok := t.(*tailer.FileTailer); ok && startOffset > 0 {
			ft.WithStartOffset(startOffset)
			fileTailerRef = ft
		} else if ft, ok := t.(*tailer.FileTailer); ok && bookmarkName != "" {
			fileTailerRef = ft
		}
		tailers = append(tailers, t)
		t.Start(ctx, logChan, errChan)
	}

	var agg *aggregator.Aggregator
	if aggregate {
		window, _ := time.ParseDuration(aggregateWindow)
		if window <= 0 {
			window = 5 * time.Second
		}
		agg = aggregator.New(window, 2)
	}

	printStartupBanner(sources)

	if showHealth {
		printHealthStatus(healthMonitor)
		startHealthUpdater(ctx, healthMonitor)
	}

	result := runPipeline(ctx, logChan, errChan, regexFilter, writer, healthMonitor, apiServer, alertEngine, agg)

	for _, t := range tailers {
		_ = t.Stop()
	}

	if bookmarkName != "" && fileTailerRef != nil && len(sources) == 1 && sources[0].Type == logline.SourceTypeFile {
		mgr, err := bookmark.NewManager()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: cannot save bookmark: %v\n", err)
		} else {
			offset := fileTailerRef.LastOffset()
			inode, _ := bookmark.GetInode(sources[0].Path)
			bm := &bookmark.Bookmark{
				File:    sources[0].Path,
				Offset:  offset,
				Inode:   inode,
				SavedAt: time.Now(),
			}
			if err := mgr.Save(bookmarkName, bm); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: cannot save bookmark: %v\n", err)
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "Bookmark %q saved at offset %d\n", bookmarkName, offset)
			}
		}
	}

	return result
}

func buildSources(cmd *cobra.Command) ([]logline.SourceConfig, *config.Config, *config.OutputsConfig, error) {
	if cfgFile != "" {
		cfg, err := config.LoadConfig(cfgFile, allowLocal)
		if err != nil {
			return nil, nil, nil, err
		}
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
		if cfg.Global.Aggregate && !cmd.Flags().Changed("aggregate") {
			aggregate = true
		}
		if cfg.Global.AggregateWindow != "" && !cmd.Flags().Changed("aggregate-window") {
			aggregateWindow = cfg.Global.AggregateWindow
		}
		return cfg.Sources, cfg, &cfg.Outputs, nil
	}

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
		jt, err := tailer.NewJournalctlTailer(src.Unit, src.Follow, monitor)
		if err != nil {
			return nil, err
		}
		if src.Priority != "" {
			jt.WithPriority(src.Priority)
		}
		if src.OutputFormat != "" {
			jt.WithOutputFormat(src.OutputFormat)
		}
		return jt, nil
	case logline.SourceTypeKubernetes:
		return tailer.NewKubernetesTailer(src.Namespace, src.Pod, src.Container, src.LabelSelector, src.Kubeconfig, src.Follow, monitor)
	case logline.SourceTypeStdin:
		return tailer.NewStdinTailer(monitor), nil
	default:
		return nil, fmt.Errorf("unsupported source type %q", src.Type)
	}
}
