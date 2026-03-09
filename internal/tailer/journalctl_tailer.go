package tailer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os/exec"
	"strings"
	"time"
)

// journalctlPriorityToLevel maps syslog priority names to logtailr log levels.
var journalctlPriorityToLevel = map[string]string{
	"0": "fatal", // emerg
	"1": "fatal", // alert
	"2": "fatal", // crit
	"3": "error", // err
	"4": "warn",  // warning
	"5": "info",  // notice
	"6": "info",  // info
	"7": "debug", // debug
}

// JournalctlTailer reads log lines from systemd journal using `journalctl`.
type JournalctlTailer struct {
	BaseTailer
	unit         string
	follow       bool
	priority     string
	outputFormat string
	cancel       context.CancelFunc
}

// NewJournalctlTailer creates a new JournalctlTailer.
func NewJournalctlTailer(unit string, follow bool, healthMonitor *health.Monitor) (*JournalctlTailer, error) {
	if err := ValidateExternalName(unit, "unit"); err != nil {
		return nil, err
	}
	name := "journalctl:" + unit
	jt := &JournalctlTailer{
		BaseTailer: BaseTailer{
			SourceName:    name,
			HealthMonitor: healthMonitor,
		},
		unit:   unit,
		follow: follow,
	}

	if healthMonitor != nil {
		healthMonitor.RegisterSource(name)
	}

	return jt, nil
}

// WithPriority sets the journalctl priority filter (e.g. "err", "warning").
func (jt *JournalctlTailer) WithPriority(priority string) {
	jt.priority = priority
}

// WithOutputFormat sets the journalctl output format ("json" for structured output).
func (jt *JournalctlTailer) WithOutputFormat(format string) {
	jt.outputFormat = format
}

// Start begins reading journal logs.
func (jt *JournalctlTailer) Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	ctx, jt.cancel = context.WithCancel(ctx)

	go jt.run(ctx, out, errChan)
}

// Stop signals the tailer to stop.
func (jt *JournalctlTailer) Stop() error {
	if jt.cancel != nil {
		jt.cancel()
	}
	jt.ReportStopped()
	return nil
}

func (jt *JournalctlTailer) run(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	args := []string{"--no-pager"}

	if jt.outputFormat == "json" {
		args = append(args, "-o", "json")
	} else {
		args = append(args, "-o", "short-iso")
	}

	if jt.unit != "" {
		args = append(args, "-u", jt.unit)
	}
	if jt.priority != "" {
		args = append(args, "-p", jt.priority)
	}
	if jt.follow {
		args = append(args, "-f")
	} else {
		args = append(args, "-n", "100")
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		jt.ReportFailed(err)
		errChan <- fmt.Errorf("journalctl stdout pipe: %w", err)
		return
	}

	if err := cmd.Start(); err != nil {
		jt.ReportFailed(err)
		errChan <- fmt.Errorf("journalctl failed for unit %q: %w", jt.unit, err)
		return
	}

	jt.ReportHealthy()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, readBufferSize), maxLineSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var ll *logline.LogLine
		if jt.outputFormat == "json" {
			ll = jt.parseJournalJSON(line)
		} else {
			ll = &logline.LogLine{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   line,
				Source:    jt.SourceName,
				Fields:    make(map[string]any),
			}
		}

		select {
		case out <- ll:
		case <-ctx.Done():
			return
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
		default:
			jt.ReportDegraded(err)
			errChan <- fmt.Errorf("journalctl read error: %w", err)
		}
	}

	if err := cmd.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:
			jt.ReportFailed(err)
			errChan <- fmt.Errorf("journalctl process exited: %w", err)
		}
	}
}

// parseJournalJSON parses a single journalctl JSON line into a LogLine.
func (jt *JournalctlTailer) parseJournalJSON(raw string) *logline.LogLine {
	var entry map[string]any
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		// Fallback to plain text if JSON parsing fails
		return &logline.LogLine{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   raw,
			Source:    jt.SourceName,
			Fields:    make(map[string]any),
		}
	}

	// Extract MESSAGE
	message, _ := entry["MESSAGE"].(string)

	// Extract timestamp from __REALTIME_TIMESTAMP (microseconds since epoch)
	ts := time.Now()
	if usec, ok := entry["__REALTIME_TIMESTAMP"].(string); ok {
		var usecInt int64
		if _, err := fmt.Sscanf(usec, "%d", &usecInt); err == nil {
			ts = time.UnixMicro(usecInt)
		}
	}

	// Map PRIORITY to log level
	level := "info"
	if prio, ok := entry["PRIORITY"].(string); ok {
		if mapped, ok := journalctlPriorityToLevel[prio]; ok {
			level = mapped
		}
	}

	// Collect useful fields
	fields := make(map[string]any)
	fieldKeys := []string{
		"_HOSTNAME", "_SYSTEMD_UNIT", "_PID", "_UID",
		"SYSLOG_IDENTIFIER", "_COMM", "_EXE",
	}
	for _, key := range fieldKeys {
		if v, ok := entry[key]; ok {
			fields[strings.ToLower(key)] = v
		}
	}

	return &logline.LogLine{
		Timestamp: ts,
		Level:     level,
		Message:   message,
		Source:    jt.SourceName,
		Fields:    fields,
	}
}
