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

// DockerTailer reads log lines from a Docker container using `docker logs`.
type DockerTailer struct {
	BaseTailer
	container string
	follow    bool
	cancel    context.CancelFunc
}

// NewDockerTailer creates a new DockerTailer.
func NewDockerTailer(container string, follow bool, healthMonitor *health.Monitor) *DockerTailer {
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

	return dt
}

// Start begins reading container logs.
func (dt *DockerTailer) Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	ctx, dt.cancel = context.WithCancel(ctx)

	go dt.run(ctx, out, errChan)
}

// Stop signals the tailer to stop.
func (dt *DockerTailer) Stop() error {
	if dt.cancel != nil {
		dt.cancel()
	}
	dt.ReportStopped()
	return nil
}

func (dt *DockerTailer) run(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
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
		return
	}

	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		dt.ReportFailed(err)
		errChan <- fmt.Errorf("docker logs failed for %q: %w", dt.container, err)
		return
	}

	dt.ReportHealthy()

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
			Source:    dt.SourceName,
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
			dt.ReportDegraded(err)
			errChan <- fmt.Errorf("docker logs read error: %w", err)
		}
	}

	// Wait for process to finish
	if err := cmd.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:
			dt.ReportFailed(err)
			errChan <- fmt.Errorf("docker logs process exited: %w", err)
		}
	}
}
