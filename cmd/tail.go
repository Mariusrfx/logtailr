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
	Short: "Tail and parse logs from a source",
	Long:  `Tail logs from a specific file with real-time parsing and filtering capabilities.`,
	RunE:  runTail,
}

func init() {
	rootCmd.AddCommand(tailCmd)

	tailCmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the log file to process")
	tailCmd.Flags().StringVarP(&level, "level", "l", "info", "Minimum log level (debug, info, warn, error, fatal)")
	tailCmd.Flags().StringVarP(&regex, "regex", "r", "", "Optional regex pattern to filter messages")
	tailCmd.Flags().BoolVar(&follow, "follow", true, "Follow the file for new lines (like tail -f)")
	tailCmd.Flags().StringVarP(&parserFlag, "parser", "p", "", "Log format parser: json, logfmt, text (default: auto-detect)")
	tailCmd.Flags().StringVarP(&outputFlag, "output", "o", "console", "Output format: console, json, file")
	tailCmd.Flags().StringVar(&outputPath, "output-path", "", "Output file path (required when --output=file)")
	tailCmd.Flags().BoolVar(&showHealth, "show-health", false, "Show health status of sources")
	tailCmd.Flags().IntVar(&healthEvery, "health-every", 0, "Show health updates every N seconds (0 = disabled)")
}

func runTail(cmd *cobra.Command, _ []string) error {
	if filePath == "" {
		return fmt.Errorf("the --file flag is required")
	}

	// Validate file path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("cannot resolve file path: %w", err)
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file")
	}
	filePath = absPath

	// Validate log level
	if _, ok := logline.LogLevels[strings.ToLower(level)]; !ok {
		return fmt.Errorf("invalid log level %q: must be one of debug, info, warn, error, fatal", level)
	}

	// Validate and compile regex upfront
	regexFilter, err := filter.NewRegexFilter(regex)
	if err != nil {
		return err
	}

	// Validate output flags
	if outputFlag == "file" && outputPath == "" {
		return fmt.Errorf("--output-path is required when --output=file")
	}

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Initialize components
	healthMonitor := health.NewMonitor()
	logParser := parser.New(filePath)
	writer, err := createWriter()
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	ft := tailer.NewFileTailer(filePath, follow, healthMonitor)

	fmt.Printf("Tailing %s | level>=%s | parser=%s | output=%s\n",
		filePath, level, parserOrAuto(), outputFlag)

	if showHealth {
		printHealthStatus(healthMonitor)
		startHealthUpdater(ctx, healthMonitor)
	}

	// Start the pipeline
	logChan := make(chan *logline.LogLine, logChannelBuffer)
	errChan := make(chan error, errChannelBuffer)

	ft.Start(ctx, logChan, errChan)

	return runPipeline(ctx, logChan, errChan, logParser, regexFilter, writer, healthMonitor)
}

func runPipeline(
	ctx context.Context,
	logChan <-chan *logline.LogLine,
	errChan <-chan error,
	logParser *parser.Parser,
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

			// Parse: convert raw line into structured log
			parsed, err := logParser.Parse(raw.Message, parserFlag)
			if err != nil {
				// Unparseable lines pass through as-is
				parsed = raw
			} else {
				parsed.Source = raw.Source
			}

			// Filter: check level
			if !filter.ByLevel(parsed, level) {
				continue
			}

			// Filter: check regex
			if !regexFilter.Match(parsed.Message) {
				continue
			}

			// Output: write to destination
			if err := writer.Write(parsed); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Output error: %v\n", err)
			}
		}
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

func parserOrAuto() string {
	if parserFlag == "" {
		return "auto"
	}
	return parserFlag
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
