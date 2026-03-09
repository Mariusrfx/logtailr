package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"logtailr/internal/health"
	"logtailr/pkg/logline"

	"github.com/spf13/cobra"
)

const (
	maxErrorMsgLen   = 20
	maxSourceNameLen = 27
)

var (
	filePath    string
	level       string
	regex       string
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
	tailCmd.Flags().BoolVar(&showHealth, "show-health", false, "Show health status of sources")
	tailCmd.Flags().IntVar(&healthEvery, "health-every", 0, "Show health updates every N seconds (0 = disabled)")
}

func runTail(cmd *cobra.Command, args []string) error {
	if filePath == "" {
		return fmt.Errorf("the --file flag is required")
	}

	// Validate file path: resolve to absolute and ensure it's a regular file
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	// Resolve symlinks to prevent symlink-based traversal
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

	// Validate regex pattern early (compile once, fail fast)
	if regex != "" {
		if _, err := regexp.Compile(regex); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	// Setup context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	healthMonitor := health.NewMonitor()
	healthMonitor.RegisterSource(filePath)
	healthMonitor.MarkHealthy(filePath)

	fmt.Printf("Starting tail on file: %s | Level: %s | Regex: %s\n", filePath, level, regex)

	if showHealth {
		printHealthStatus(healthMonitor)
		startHealthUpdater(ctx, healthMonitor)
	}

	// TODO: Logic orchestration will go here
	fmt.Println("\n" + healthMonitor.Summary())

	return nil
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
	defer w.Flush()

	for _, s := range statuses {
		fmt.Fprintf(w, "│ %-27s │ %-9s │ %-12d │ %-20s │\n",
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
