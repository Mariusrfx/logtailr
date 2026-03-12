package alert

import (
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"sync"
	"testing"
	"time"
)

type mockNotifier struct {
	mu     sync.Mutex
	events []*Event
}

func (m *mockNotifier) Notify(event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockNotifier) Close() error { return nil }

func (m *mockNotifier) getEvents() []*Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Event, len(m.events))
	copy(result, m.events)
	return result
}

func waitForEvents(notifier *mockNotifier, count int, timeout time.Duration) []*Event {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		events := notifier.getEvents()
		if len(events) >= count {
			return events
		}
		time.Sleep(10 * time.Millisecond)
	}
	return notifier.getEvents()
}

func TestPatternRule(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "oom-detect",
			Type:     RuleTypePattern,
			Severity: SeverityCritical,
			Pattern:  "OutOfMemory|OOM",
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessLine(&logline.LogLine{
		Source:    "app",
		Level:     "error",
		Message:   "java.lang.OutOfMemoryError: heap space",
		Timestamp: time.Now(),
	})

	engine.ProcessLine(&logline.LogLine{
		Source:    "app",
		Level:     "info",
		Message:   "Application started successfully",
		Timestamp: time.Now(),
	})

	events := waitForEvents(notifier, 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Rule != "oom-detect" {
		t.Errorf("expected rule 'oom-detect', got %q", events[0].Rule)
	}
	if events[0].Severity != string(SeverityCritical) {
		t.Errorf("expected severity critical, got %q", events[0].Severity)
	}
}

func TestPatternRule_SourceFilter(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "app-errors",
			Type:     RuleTypePattern,
			Severity: SeverityWarning,
			Pattern:  "ERROR",
			Source:   "app-logs",
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessLine(&logline.LogLine{
		Source:  "other",
		Level:   "error",
		Message: "ERROR in processing",
	})

	engine.ProcessLine(&logline.LogLine{
		Source:  "app-logs",
		Level:   "error",
		Message: "ERROR in processing",
	})

	events := waitForEvents(notifier, 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Source != "app-logs" {
		t.Errorf("expected source 'app-logs', got %q", events[0].Source)
	}
}

func TestLevelRule(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "fatal-alert",
			Type:     RuleTypeLevel,
			Severity: SeverityCritical,
			Level:    "fatal",
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessLine(&logline.LogLine{
		Source: "app", Level: "error", Message: "some error",
	})

	engine.ProcessLine(&logline.LogLine{
		Source: "app", Level: "fatal", Message: "crash",
	})

	events := waitForEvents(notifier, 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Rule != "fatal-alert" {
		t.Errorf("expected rule 'fatal-alert', got %q", events[0].Rule)
	}
}

func TestErrorRateRule(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:      "high-errors",
			Type:      RuleTypeErrorRate,
			Severity:  SeverityWarning,
			Threshold: 3,
			Window:    5 * time.Minute,
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		engine.ProcessLine(&logline.LogLine{
			Source: "app", Level: "error", Message: "db connection failed",
		})
	}

	events := waitForEvents(notifier, 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Count != 3 {
		t.Errorf("expected count 3, got %d", events[0].Count)
	}
}

func TestErrorRateRule_InfoNotCounted(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:      "errors-only",
			Type:      RuleTypeErrorRate,
			Severity:  SeverityWarning,
			Threshold: 3,
			Window:    5 * time.Minute,
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	for i := 0; i < 10; i++ {
		engine.ProcessLine(&logline.LogLine{
			Source: "app", Level: "info", Message: "request processed",
		})
	}

	time.Sleep(200 * time.Millisecond)

	events := notifier.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestHealthChangeRule(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "source-down",
			Type:     RuleTypeHealthChange,
			Severity: SeverityCritical,
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessHealthChange("db-logs", health.StatusHealthy, health.StatusFailed)

	events := notifier.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Source != "db-logs" {
		t.Errorf("expected source 'db-logs', got %q", events[0].Source)
	}
}

func TestHealthChangeRule_SourceFilter(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "specific-source",
			Type:     RuleTypeHealthChange,
			Severity: SeverityCritical,
			Source:   "important-app",
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessHealthChange("other", health.StatusHealthy, health.StatusFailed)
	engine.ProcessHealthChange("important-app", health.StatusHealthy, health.StatusFailed)

	events := notifier.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestCooldown(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "level-with-cooldown",
			Type:     RuleTypeLevel,
			Severity: SeverityWarning,
			Level:    "error",
			Cooldown: 1 * time.Second,
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessLine(&logline.LogLine{
		Source: "app", Level: "error", Message: "first",
	})
	engine.ProcessLine(&logline.LogLine{
		Source: "app", Level: "error", Message: "second",
	})

	events := waitForEvents(notifier, 1, 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	events = notifier.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event (cooldown), got %d", len(events))
	}
}

func TestRecentEvents(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "level-alert",
			Type:     RuleTypeLevel,
			Severity: SeverityWarning,
			Level:    "error",
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	for i := 0; i < 5; i++ {
		engine.ProcessLine(&logline.LogLine{
			Source: "app", Level: "error", Message: "error msg",
		})
	}

	waitForEvents(notifier, 5, 2*time.Second)

	events := engine.RecentEvents(3)
	if len(events) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(events))
	}

	all := engine.RecentEvents(0)
	if len(all) != 5 {
		t.Fatalf("expected 5 total events, got %d", len(all))
	}
}

func TestRuleStats(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{
			Name:     "test-rule",
			Type:     RuleTypeLevel,
			Severity: SeverityWarning,
			Level:    "error",
		},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	engine.ProcessLine(&logline.LogLine{
		Source: "app", Level: "error", Message: "err",
	})

	waitForEvents(notifier, 1, 2*time.Second)

	stats := engine.RuleStats()
	st, ok := stats["test-rule"]
	if !ok {
		t.Fatal("expected stats for 'test-rule'")
	}
	if st.FireCount != 1 {
		t.Errorf("expected fire_count 1, got %d", st.FireCount)
	}
	if st.LastFired.IsZero() {
		t.Error("expected last_fired to be set")
	}
}

func TestInvalidRule(t *testing.T) {
	_, err := NewEngine([]Rule{
		{Name: "bad", Type: "unknown", Severity: SeverityWarning},
	}, nil)
	if err == nil {
		t.Fatal("expected error for unknown rule type")
	}
}

func TestInvalidPatternRule(t *testing.T) {
	_, err := NewEngine([]Rule{
		{Name: "bad-regex", Type: RuleTypePattern, Severity: SeverityWarning, Pattern: "[invalid"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func TestInvalidLevelRule(t *testing.T) {
	_, err := NewEngine([]Rule{
		{Name: "bad-level", Type: RuleTypeLevel, Severity: SeverityWarning, Level: "invalid"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestProcessLineNonBlocking(t *testing.T) {
	notifier := &mockNotifier{}
	engine, err := NewEngine([]Rule{
		{Name: "test", Type: RuleTypeLevel, Severity: SeverityWarning, Level: "error"},
	}, []Notifier{notifier})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	for i := 0; i < processQueueSize+100; i++ {
		engine.ProcessLine(&logline.LogLine{
			Source: "app", Level: "error", Message: "msg",
		})
	}
}
