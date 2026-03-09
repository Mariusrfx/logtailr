package tailer

import (
	"bufio"
	"context"
	"fmt"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os/exec"
	"time"
)

// JournalctlTailer reads log lines from systemd journal using `journalctl`.
type JournalctlTailer struct {
	BaseTailer
	unit   string
	follow bool
	cancel context.CancelFunc
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
	args := []string{"--no-pager", "-o", "short-iso"}
	if jt.unit != "" {
		args = append(args, "-u", jt.unit)
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

		ll := &logline.LogLine{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   line,
			Source:    jt.SourceName,
			Fields:    make(map[string]interface{}),
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
