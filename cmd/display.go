package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"logtailr/internal/health"
	"logtailr/pkg/logline"
)

const (
	maxErrorMsgLen   = 20
	maxSourceNameLen = 27
)

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
