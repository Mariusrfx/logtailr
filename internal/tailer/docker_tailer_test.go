package tailer

import (
	"logtailr/internal/health"
	"testing"
	"time"
)

func TestDockerTailer_ValidContainer(t *testing.T) {
	monitor := health.NewMonitor()
	dt, err := NewDockerTailer("my-app-container", true, monitor)
	if err != nil {
		t.Fatalf("NewDockerTailer() error = %v", err)
	}
	if dt.GetSourceName() != "docker:my-app-container" {
		t.Errorf("source name = %q, want %q", dt.GetSourceName(), "docker:my-app-container")
	}
}

func TestDockerTailer_InvalidContainer(t *testing.T) {
	_, err := NewDockerTailer("../../etc/passwd", true, nil)
	if err == nil {
		t.Fatal("expected error for invalid container name")
	}
}

func TestDockerTailer_EmptyContainer(t *testing.T) {
	_, err := NewDockerTailer("", true, nil)
	if err == nil {
		t.Fatal("expected error for empty container name")
	}
}

func TestParseDockerLine_WithTimestamp(t *testing.T) {
	line := "2026-03-09T13:35:54.777104285Z Terraform v1.9.7 on linux_amd64"
	ts, msg := parseDockerLine(line)

	if ts.Year() != 2026 || ts.Month() != 3 || ts.Day() != 9 {
		t.Errorf("timestamp = %v, want 2026-03-09", ts)
	}
	if msg != "Terraform v1.9.7 on linux_amd64" {
		t.Errorf("msg = %q, want %q", msg, "Terraform v1.9.7 on linux_amd64")
	}
}

func TestParseDockerLine_WithANSI(t *testing.T) {
	line := "2026-03-09T13:35:54.777104285Z \x1b[31m│\x1b[0m \x1b[0m\x1b[1m\x1b[31mError: \x1b[0m\x1b[0m\x1b[1mInvalid Attribute\x1b[0m"
	ts, msg := parseDockerLine(line)

	if ts.Year() != 2026 {
		t.Errorf("timestamp = %v, want 2026", ts)
	}
	if msg != "│ Error: Invalid Attribute" {
		t.Errorf("msg = %q, want %q", msg, "│ Error: Invalid Attribute")
	}
}

func TestParseDockerLine_NoTimestamp(t *testing.T) {
	line := "just a plain message"
	before := time.Now()
	ts, msg := parseDockerLine(line)

	if ts.Before(before) {
		t.Error("timestamp should be time.Now() when no docker timestamp")
	}
	if msg != "just a plain message" {
		t.Errorf("msg = %q, want %q", msg, "just a plain message")
	}
}

func TestParseDockerLine_EmptyAfterStrip(t *testing.T) {
	line := "2026-03-09T13:35:54.777104285Z \x1b[31m╵\x1b[0m\x1b[0m"
	_, msg := parseDockerLine(line)

	if msg != "╵" {
		t.Errorf("msg = %q, want %q", msg, "╵")
	}
}

func TestDockerTailer_Stop(t *testing.T) {
	monitor := health.NewMonitor()
	dt, err := NewDockerTailer("test-container", true, monitor)
	if err != nil {
		t.Fatalf("NewDockerTailer() error = %v", err)
	}

	if err := dt.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	status, ok := monitor.GetStatus("docker:test-container")
	if !ok {
		t.Fatal("expected source to be registered")
	}
	if status.Status != health.StatusStopped {
		t.Errorf("status = %q, want %q", status.Status, health.StatusStopped)
	}
}
