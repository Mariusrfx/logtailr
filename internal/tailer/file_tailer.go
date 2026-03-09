package tailer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	reopenDelay    = 500 * time.Millisecond
	reopenMaxRetry = 10
	readBufferSize = 64 * 1024
	maxLineSize    = 256 * 1024 // 256KB max per log line
)

// FileTailer reads log lines from a file, optionally following new writes.
type FileTailer struct {
	BaseTailer
	path   string
	follow bool
	cancel context.CancelFunc
}

// NewFileTailer creates a new FileTailer.
func NewFileTailer(path string, follow bool, healthMonitor *health.Monitor) *FileTailer {
	ft := &FileTailer{
		BaseTailer: BaseTailer{
			SourceName:    path,
			HealthMonitor: healthMonitor,
		},
		path:   path,
		follow: follow,
	}

	if healthMonitor != nil {
		healthMonitor.RegisterSource(path)
	}

	return ft
}

// Start begins tailing the file. It reads existing content and, if follow is
// true, watches for new lines using fsnotify.
func (ft *FileTailer) Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	ctx, ft.cancel = context.WithCancel(ctx)

	go ft.run(ctx, out, errChan)
}

// Stop signals the tailer to stop and marks the source as stopped.
func (ft *FileTailer) Stop() error {
	if ft.cancel != nil {
		ft.cancel()
	}
	ft.ReportStopped()
	return nil
}

func (ft *FileTailer) run(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	file, err := os.Open(ft.path)
	if err != nil {
		ft.ReportFailed(err)
		errChan <- fmt.Errorf("failed to open log source: %w", err)
		return
	}
	defer func() { _ = file.Close() }()

	ft.ReportHealthy()

	reader := bufio.NewReaderSize(file, readBufferSize)

	// Read existing content
	if err := ft.readLines(ctx, reader, out); err != nil {
		return
	}

	if !ft.follow {
		return
	}

	ft.followFile(ctx, file, reader, out, errChan)
}

// readLines reads all available complete lines from the reader.
func (ft *FileTailer) readLines(ctx context.Context, reader *bufio.Reader, out chan<- *logline.LogLine) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			// Trim the newline but keep the content
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			// Enforce max line size to prevent memory exhaustion
			if len(line) > maxLineSize {
				line = line[:maxLineSize]
			}
			if line != "" {
				ll := &logline.LogLine{
					Timestamp: time.Now(),
					Level:     "info",
					Message:   line,
					Source:    ft.SourceName,
					Fields:    make(map[string]interface{}),
				}
				select {
				case out <- ll:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// followFile watches the file for new writes or rename/remove events.
func (ft *FileTailer) followFile(ctx context.Context, file *os.File, reader *bufio.Reader, out chan<- *logline.LogLine, errChan chan<- error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		ft.ReportFailed(err)
		errChan <- fmt.Errorf("failed to create file watcher: %w", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(ft.path); err != nil {
		ft.ReportFailed(err)
		errChan <- fmt.Errorf("failed to watch log source: %w", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) {
				if err := ft.readLines(ctx, reader, out); err != nil {
					return
				}
				ft.ReportHealthy()
			}

			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				ft.ReportDegraded(fmt.Errorf("file rotated or removed"))

				// Try to reopen (logrotate scenario)
				newFile, newReader, err := ft.reopenFile(ctx, watcher)
				if err != nil {
					ft.ReportFailed(err)
					errChan <- fmt.Errorf("failed to reopen log source after rotation: %w", err)
					return
				}

				if closeErr := file.Close(); closeErr != nil {
					ft.ReportDegraded(fmt.Errorf("failed to close old file handle: %w", closeErr))
				}
				file = newFile
				reader = newReader
				ft.ReportHealthy()
			}

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return
			}
			ft.ReportDegraded(watchErr)
			errChan <- fmt.Errorf("file watcher error: %w", watchErr)
		}
	}
}

// reopenFile attempts to reopen the file after rotation, with retries.
func (ft *FileTailer) reopenFile(ctx context.Context, watcher *fsnotify.Watcher) (*os.File, *bufio.Reader, error) {
	for i := range reopenMaxRetry {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(reopenDelay):
		}

		file, err := os.Open(ft.path)
		if err != nil {
			if i == reopenMaxRetry-1 {
				return nil, nil, fmt.Errorf("failed to reopen after %d retries: %w", reopenMaxRetry, err)
			}
			continue
		}

		// Re-add watcher on the new file
		if err := watcher.Remove(ft.path); err != nil {
			ft.ReportDegraded(fmt.Errorf("failed to remove old watcher: %w", err))
		}
		if err := watcher.Add(ft.path); err != nil {
			_ = file.Close()
			return nil, nil, fmt.Errorf("failed to re-watch after rotation: %w", err)
		}

		return file, bufio.NewReaderSize(file, readBufferSize), nil
	}

	return nil, nil, fmt.Errorf("max retries exceeded for file reopen")
}
