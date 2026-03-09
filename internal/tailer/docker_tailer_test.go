package tailer

import (
	"logtailr/internal/health"
	"testing"
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
