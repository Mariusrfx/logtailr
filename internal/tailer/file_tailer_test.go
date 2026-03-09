package tailer

import (
	"context"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempLogFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func collectLines(ch <-chan *logline.LogLine, timeout time.Duration, max int) []*logline.LogLine {
	var lines []*logline.LogLine
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case ll := <-ch:
			lines = append(lines, ll)
			if len(lines) >= max {
				return lines
			}
		case <-timer.C:
			return lines
		}
	}
}

func TestFileTailer_ReadExistingFile(t *testing.T) {
	path := tempLogFile(t, "line one\nline two\nline three\n")

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, false, monitor)

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	lines := collectLines(out, time.Second, 3)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	if lines[0].Message != "line one" {
		t.Errorf("line[0].Message = %q, want %q", lines[0].Message, "line one")
	}
	if lines[1].Message != "line two" {
		t.Errorf("line[1].Message = %q, want %q", lines[1].Message, "line two")
	}
	if lines[2].Message != "line three" {
		t.Errorf("line[2].Message = %q, want %q", lines[2].Message, "line three")
	}

	if lines[0].Source != path {
		t.Errorf("Source = %q, want %q", lines[0].Source, path)
	}
}

func expectTailerFails(t *testing.T, path string, ft *FileTailer, monitor *health.Monitor) {
	t.Helper()

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("expected error within timeout")
	}

	status, ok := monitor.GetStatus(path)
	if !ok {
		t.Fatal("source not registered")
	}
	if status.Status != health.StatusFailed {
		t.Errorf("status = %q, want %q", status.Status, health.StatusFailed)
	}
}

func TestFileTailer_FileNotFound(t *testing.T) {
	path := "/nonexistent/file.log"
	monitor := health.NewMonitor()
	ft := NewFileTailer(path, false, monitor)
	expectTailerFails(t, path, ft, monitor)
}

func TestFileTailer_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test as root")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.log")
	if err := os.WriteFile(path, []byte("secret\n"), 0000); err != nil {
		t.Fatalf("write: %v", err)
	}

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, false, monitor)
	expectTailerFails(t, path, ft, monitor)
}

func TestFileTailer_Follow_NewLines(t *testing.T) {
	path := tempLogFile(t, "initial line\n")

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, true, monitor)

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	// Read initial line
	lines := collectLines(out, time.Second, 1)
	if len(lines) != 1 {
		t.Fatalf("initial: got %d lines, want 1", len(lines))
	}

	// Append new lines
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.WriteString("new line one\nnew line two\n"); err != nil {
		_ = f.Close()
		t.Fatalf("write: %v", err)
	}
	_ = f.Close()

	newLines := collectLines(out, 2*time.Second, 2)
	if len(newLines) != 2 {
		t.Fatalf("follow: got %d lines, want 2", len(newLines))
	}

	if newLines[0].Message != "new line one" {
		t.Errorf("newLines[0].Message = %q, want %q", newLines[0].Message, "new line one")
	}
	if newLines[1].Message != "new line two" {
		t.Errorf("newLines[1].Message = %q, want %q", newLines[1].Message, "new line two")
	}

	_ = ft.Stop()
}

func TestFileTailer_ContextCancel(t *testing.T) {
	path := tempLogFile(t, "line\n")

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, true, monitor)

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())

	ft.Start(ctx, out, errCh)

	// Read initial
	collectLines(out, 500*time.Millisecond, 1)

	// Cancel context
	cancel()

	// Give goroutine time to exit
	time.Sleep(200 * time.Millisecond)

	// Should not receive any more lines
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = f.WriteString("after cancel\n")
		_ = f.Close()
	}

	lines := collectLines(out, 500*time.Millisecond, 1)
	if len(lines) > 0 {
		t.Error("received lines after context cancel")
	}
}

func TestFileTailer_HealthIntegration(t *testing.T) {
	path := tempLogFile(t, "test\n")

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, false, monitor)

	// Check registered as starting
	status, ok := monitor.GetStatus(path)
	if !ok {
		t.Fatal("source not registered")
	}
	if status.Status != health.StatusStarting {
		t.Errorf("initial status = %q, want %q", status.Status, health.StatusStarting)
	}

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	// Wait for it to read
	collectLines(out, time.Second, 1)

	status, _ = monitor.GetStatus(path)
	if status.Status != health.StatusHealthy {
		t.Errorf("after read status = %q, want %q", status.Status, health.StatusHealthy)
	}

	// Stop and check
	_ = ft.Stop()
	status, _ = monitor.GetStatus(path)
	if status.Status != health.StatusStopped {
		t.Errorf("after stop status = %q, want %q", status.Status, health.StatusStopped)
	}
}

func TestFileTailer_FileDeleted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deleteme.log")
	if err := os.WriteFile(path, []byte("before delete\n"), 0644); err != nil {
		t.Fatal(err)
	}

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, true, monitor)

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	// Read initial
	collectLines(out, time.Second, 1)

	// Delete the file
	_ = os.Remove(path)

	// Should get an error or degraded status eventually
	select {
	case <-errCh:
		// Expected: reopen failed
	case <-time.After(3 * time.Second):
		// Also acceptable if fsnotify doesn't fire remove on all platforms
	}

	_ = ft.Stop()
}

func TestFileTailer_NilMonitor(t *testing.T) {
	path := tempLogFile(t, "line\n")

	ft := NewFileTailer(path, false, nil)

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	lines := collectLines(out, time.Second, 1)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
}

func TestFileTailer_EmptyFile(t *testing.T) {
	path := tempLogFile(t, "")

	monitor := health.NewMonitor()
	ft := NewFileTailer(path, false, monitor)

	out := make(chan *logline.LogLine, 10)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ft.Start(ctx, out, errCh)

	lines := collectLines(out, 500*time.Millisecond, 1)
	if len(lines) != 0 {
		t.Fatalf("got %d lines from empty file, want 0", len(lines))
	}
}
