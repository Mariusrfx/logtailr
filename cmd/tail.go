package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"logtailr/internal/health"

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
	tailCmd.Flags().StringVarP(&level, "level", "l", "info", "Minimum log level (debug, info, warn, error)")
	tailCmd.Flags().StringVarP(&regex, "regex", "r", "", "Optional regex pattern to filter messages")
	tailCmd.Flags().BoolVar(&showHealth, "show-health", false, "Show health status of sources")
	tailCmd.Flags().IntVar(&healthEvery, "health-every", 0, "Show health updates every N seconds (0 = disabled)")
}

func runTail(cmd *cobra.Command, args []string) error {
	if filePath == "" {
		return fmt.Errorf("the --file flag is required")
	}

	healthMonitor := health.NewMonitor()
	healthMonitor.RegisterSource(filePath)
	healthMonitor.MarkHealthy(filePath)

	fmt.Printf("Starting tail on file: %s | Level: %s | Regex: %s\n", filePath, level, regex)

	if showHealth {
		printHealthStatus(healthMonitor)
		startHealthUpdater(healthMonitor)
	}

	// TODO: Logic orchestration will go here
	fmt.Println("\n" + healthMonitor.Summary())

	return nil
}

func startHealthUpdater(monitor *health.Monitor) {
	if healthEvery <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Duration(healthEvery) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			fmt.Println("\n--- Health Update ---")
			printHealthStatus(monitor)
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
