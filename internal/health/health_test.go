package health

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMonitorRegisterSource(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("test-source")

	status, exists := monitor.GetStatus("test-source")

	assertExists(t, exists, "test-source")
	assertEqual(t, status.Status, StatusStarting)
	assertEqual(t, status.Name, "test-source")
}

func TestMonitorUpdateStatus(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("test-source")

	testErr := errors.New("test error")
	monitor.UpdateStatus("test-source", StatusFailed, testErr)

	status, _ := monitor.GetStatus("test-source")

	assertEqual(t, status.Status, StatusFailed)
	assertEqual(t, status.ErrorCount, 1)

	if status.LastError == nil || status.LastError.Error() != "test error" {
		t.Error("Expected error to be set")
	}
}

func TestMonitorMarkHealthy(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("test-source")

	monitor.MarkHealthy("test-source")

	status, _ := monitor.GetStatus("test-source")
	assertEqual(t, status.Status, StatusHealthy)
}

func TestMonitorMarkFailed(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("test-source")

	monitor.MarkFailed("test-source", errors.New("failure"))

	status, _ := monitor.GetStatus("test-source")
	assertEqual(t, status.Status, StatusFailed)
}

func TestMonitorMarkDegraded(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("test-source")

	monitor.MarkDegraded("test-source", errors.New("degraded"))

	status, _ := monitor.GetStatus("test-source")
	assertEqual(t, status.Status, StatusDegraded)
}

func TestMonitorMarkStopped(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("test-source")

	monitor.MarkStopped("test-source")

	status, _ := monitor.GetStatus("test-source")
	assertEqual(t, status.Status, StatusStopped)
}

func TestMonitorGetAllStatuses(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("source1")
	monitor.RegisterSource("source2")
	monitor.MarkHealthy("source1")
	monitor.MarkFailed("source2", errors.New("error"))

	statuses := monitor.GetAllStatuses()

	assertEqual(t, len(statuses), 2)
	assertEqual(t, statuses["source1"].Status, StatusHealthy)
	assertEqual(t, statuses["source2"].Status, StatusFailed)
}

func TestMonitorGetHealthySources(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("healthy1")
	monitor.RegisterSource("healthy2")
	monitor.RegisterSource("failed1")

	monitor.MarkHealthy("healthy1")
	monitor.MarkHealthy("healthy2")
	monitor.MarkFailed("failed1", errors.New("error"))

	healthy := monitor.GetHealthySources()

	assertEqual(t, len(healthy), 2)
}

func TestMonitorGetFailedSources(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("healthy1")
	monitor.RegisterSource("failed1")
	monitor.RegisterSource("failed2")

	monitor.MarkHealthy("healthy1")
	monitor.MarkFailed("failed1", errors.New("error1"))
	monitor.MarkFailed("failed2", errors.New("error2"))

	failed := monitor.GetFailedSources()

	assertEqual(t, len(failed), 2)
}

func TestMonitorGetHealthCount(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("healthy")
	monitor.RegisterSource("degraded")
	monitor.RegisterSource("failed")
	monitor.RegisterSource("stopped")

	monitor.MarkHealthy("healthy")
	monitor.MarkDegraded("degraded", errors.New("warning"))
	monitor.MarkFailed("failed", errors.New("error"))
	monitor.MarkStopped("stopped")

	healthy, degraded, failed, stopped := monitor.GetHealthCount()

	assertEqual(t, healthy, 1)
	assertEqual(t, degraded, 1)
	assertEqual(t, failed, 1)
	assertEqual(t, stopped, 1)
}

func TestMonitorIsAllHealthy(t *testing.T) {
	t.Run("all healthy", func(t *testing.T) {
		monitor := NewMonitor()
		monitor.RegisterSource("source1")
		monitor.RegisterSource("source2")
		monitor.MarkHealthy("source1")
		monitor.MarkHealthy("source2")

		if !monitor.IsAllHealthy() {
			t.Error("Expected all sources to be healthy")
		}
	})

	t.Run("one failed", func(t *testing.T) {
		monitor := NewMonitor()
		monitor.RegisterSource("source1")
		monitor.RegisterSource("source2")
		monitor.MarkHealthy("source1")
		monitor.MarkFailed("source2", errors.New("error"))

		if monitor.IsAllHealthy() {
			t.Error("Expected not all sources to be healthy")
		}
	})
}

func TestMonitorSummary(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("healthy")
	monitor.RegisterSource("failed")
	monitor.MarkHealthy("healthy")
	monitor.MarkFailed("failed", errors.New("error"))

	summary := monitor.Summary()

	expectedParts := []string{"2 total", "1 healthy", "1 failed"}
	for _, part := range expectedParts {
		if !strings.Contains(summary, part) {
			t.Errorf("Summary should contain %q, got: %s", part, summary)
		}
	}
}

func TestStatusSymbol(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusHealthy, "✓"},
		{StatusDegraded, "⚠"},
		{StatusFailed, "✗"},
		{StatusStopped, "⏸"},
		{StatusStarting, "⏳"},
		{Status("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := tt.status.Symbol()
			if got != tt.want {
				t.Errorf("Symbol() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMonitorConcurrency(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterSource("concurrent-source")

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				monitor.MarkHealthy("concurrent-source")
				monitor.GetStatus("concurrent-source")
			}
		}()
	}

	wg.Wait()

	status, exists := monitor.GetStatus("concurrent-source")

	assertExists(t, exists, "concurrent-source")
	assertEqual(t, status.Status, StatusHealthy)
}

func TestMonitorAutoRegister(t *testing.T) {
	monitor := NewMonitor()

	monitor.UpdateStatus("auto-registered", StatusHealthy, nil)

	status, exists := monitor.GetStatus("auto-registered")

	assertExists(t, exists, "auto-registered")
	assertEqual(t, status.Status, StatusHealthy)
}

func TestSourceHealthTimestamps(t *testing.T) {
	monitor := NewMonitor()

	beforeRegister := time.Now()
	monitor.RegisterSource("test-source")
	afterRegister := time.Now()

	status, _ := monitor.GetStatus("test-source")

	assertTimeInRange(t, status.StartTime, beforeRegister, afterRegister, "StartTime")

	time.Sleep(10 * time.Millisecond)

	beforeUpdate := time.Now()
	monitor.MarkHealthy("test-source")
	afterUpdate := time.Now()

	status, _ = monitor.GetStatus("test-source")

	assertTimeInRange(t, status.LastUpdate, beforeUpdate, afterUpdate, "LastUpdate")

	if !status.LastUpdate.After(status.StartTime) {
		t.Error("LastUpdate should be after StartTime")
	}
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func assertExists(t *testing.T, exists bool, name string) {
	t.Helper()
	if !exists {
		t.Errorf("expected %q to exist", name)
	}
}

func assertTimeInRange(t *testing.T, got, start, end time.Time, name string) {
	t.Helper()
	if got.Before(start) || got.After(end) {
		t.Errorf("%s should be between %v and %v, got %v", name, start, end, got)
	}
}
