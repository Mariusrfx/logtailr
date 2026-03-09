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

const (
	reconnectBaseDelay = 1 * time.Second
	reconnectMaxDelay  = 30 * time.Second
)

// DockerTailer reads log lines from a Docker container using `docker logs`.
type DockerTailer struct {
	BaseTailer
	container string
	follow    bool
	cancel    context.CancelFunc
}

// NewDockerTailer creates a new DockerTailer.
func NewDockerTailer(container string, follow bool, healthMonitor *health.Monitor) (*DockerTailer, error) {
	if err := ValidateExternalName(container, "container"); err != nil {
		return nil, err
	}
	name := "docker:" + container
	dt := &DockerTailer{
		BaseTailer: BaseTailer{
			SourceName:    name,
			HealthMonitor: healthMonitor,
		},
		container: container,
		follow:    follow,
	}

	if healthMonitor != nil {
		healthMonitor.RegisterSource(name)
	}

	return dt, nil
}

// Start begins reading container logs.
func (dt *DockerTailer) Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	ctx, dt.cancel = context.WithCancel(ctx)

	go dt.runWithReconnect(ctx, out, errChan)
}

// Stop signals the tailer to stop.
func (dt *DockerTailer) Stop() error {
	if dt.cancel != nil {
		dt.cancel()
	}
	dt.ReportStopped()
	return nil
}

func (dt *DockerTailer) runWithReconnect(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	delay := reconnectBaseDelay

	for {
		exited := dt.run(ctx, out, errChan)

		// If context was cancelled, stop reconnecting
		if ctx.Err() != nil {
			return
		}

		// If run returned false, the process did not start at all (fatal)
		if !exited {
			return
		}

		// Container exited — attempt reconnect with backoff
		dt.ReportDegraded(fmt.Errorf("container %q exited, reconnecting in %s", dt.container, delay))
		errChan <- fmt.Errorf("docker: container %q exited, reconnecting in %s", dt.container, delay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay = delay * 2
		if delay > reconnectMaxDelay {
			delay = reconnectMaxDelay
		}
	}
}

// run executes a single docker logs session. Returns true if the process started
// and then exited (eligible for reconnect), false if it failed to start.
func (dt *DockerTailer) run(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) bool {
	args := []string{"logs", "--timestamps"}
	if dt.follow {
		args = append(args, "-f")
	}
	args = append(args, dt.container)

	cmd := exec.CommandContext(ctx, "docker", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		dt.ReportFailed(err)
		errChan <- fmt.Errorf("docker stdout pipe: %w", err)
		return false
	}

	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		dt.ReportFailed(err)
		errChan <- fmt.Errorf("docker logs failed for %q: %w", dt.container, err)
		return false
	}

	dt.ReportHealthy()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, readBufferSize), maxLineSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return true
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
			Source:    dt.SourceName,
			Fields:    make(map[string]interface{}),
		}

		select {
		case out <- ll:
		case <-ctx.Done():
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
		default:
			dt.ReportDegraded(err)
			errChan <- fmt.Errorf("docker logs read error: %w", err)
		}
	}

	// Wait for process to finish
	if err := cmd.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:
			// Process exited — eligible for reconnect
		}
	}

	return true
}
