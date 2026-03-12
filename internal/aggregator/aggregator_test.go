package aggregator

import (
	"logtailr/pkg/logline"
	"strings"
	"testing"
	"time"
)

func makeLine(source, level, message string) *logline.LogLine {
	return &logline.LogLine{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
		Fields:    make(map[string]interface{}),
	}
}

func TestSingleLinePassesThrough(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 2)
	defer agg.Stop()

	results := agg.Process(makeLine("app", "info", "hello"))
	if len(results) != 0 {
		t.Fatalf("expected 0 results from Process, got %d", len(results))
	}

	flushed := agg.Flush()
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed line, got %d", len(flushed))
	}
	if flushed[0].Count != 1 {
		t.Errorf("expected count=1, got %d", flushed[0].Count)
	}
	if flushed[0].Line.Message != "hello" {
		t.Errorf("expected message 'hello', got %q", flushed[0].Line.Message)
	}
}

func TestDuplicatesAggregate(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 2)
	defer agg.Stop()

	for i := 0; i < 3; i++ {
		results := agg.Process(makeLine("app", "error", "Connection timeout"))
		if len(results) != 0 {
			t.Fatalf("iteration %d: expected 0 results, got %d", i, len(results))
		}
	}

	flushed := agg.Flush()
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed line, got %d", len(flushed))
	}
	if flushed[0].Count != 3 {
		t.Errorf("expected count=3, got %d", flushed[0].Count)
	}
	if !strings.Contains(flushed[0].Line.Message, "(x3 in last") {
		t.Errorf("expected aggregation suffix, got %q", flushed[0].Line.Message)
	}
}

func TestDifferentMessagesDoNotAggregate(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 2)
	defer agg.Stop()

	agg.Process(makeLine("app", "info", "message A"))
	agg.Process(makeLine("app", "info", "message B"))
	agg.Process(makeLine("app", "info", "message C"))

	flushed := agg.Flush()
	if len(flushed) != 3 {
		t.Fatalf("expected 3 flushed lines, got %d", len(flushed))
	}
	for _, f := range flushed {
		if f.Count != 1 {
			t.Errorf("expected count=1, got %d for message %q", f.Count, f.Line.Message)
		}
	}
}

func TestWindowExpiry(t *testing.T) {
	now := time.Now()
	agg := newTestAggregator(2*time.Second, 2)
	defer agg.Stop()

	agg.nowFunc = func() time.Time { return now }

	agg.Process(makeLine("app", "error", "timeout"))

	// Advance past window
	agg.nowFunc = func() time.Time { return now.Add(3 * time.Second) }

	results := agg.Process(makeLine("app", "error", "timeout"))

	// Should have flushed the old entry
	found := false
	for _, r := range results {
		if r.Line.Message == "timeout" && r.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected expired entry to be flushed on next Process call")
	}

	// New entry should exist
	flushed := agg.Flush()
	if len(flushed) != 1 {
		t.Fatalf("expected 1 entry after expiry reset, got %d", len(flushed))
	}
}

func TestDifferentSourcesDoNotMix(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 2)
	defer agg.Stop()

	agg.Process(makeLine("app1", "error", "timeout"))
	agg.Process(makeLine("app2", "error", "timeout"))

	flushed := agg.Flush()
	if len(flushed) != 2 {
		t.Fatalf("expected 2 flushed lines, got %d", len(flushed))
	}
	for _, f := range flushed {
		if f.Count != 1 {
			t.Errorf("expected count=1, got %d", f.Count)
		}
	}
}

func TestDifferentLevelsDoNotMix(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 2)
	defer agg.Stop()

	agg.Process(makeLine("app", "error", "timeout"))
	agg.Process(makeLine("app", "warn", "timeout"))

	flushed := agg.Flush()
	if len(flushed) != 2 {
		t.Fatalf("expected 2 flushed lines, got %d", len(flushed))
	}
}

func TestFlushEmptiesMap(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 2)
	defer agg.Stop()

	agg.Process(makeLine("app", "info", "hello"))

	flushed1 := agg.Flush()
	if len(flushed1) != 1 {
		t.Fatalf("expected 1 flushed, got %d", len(flushed1))
	}

	flushed2 := agg.Flush()
	if len(flushed2) != 0 {
		t.Fatalf("expected 0 after second flush, got %d", len(flushed2))
	}
}

func TestMinCountThreshold(t *testing.T) {
	agg := newTestAggregator(10*time.Second, 3)
	defer agg.Stop()

	agg.Process(makeLine("app", "error", "timeout"))
	agg.Process(makeLine("app", "error", "timeout"))

	flushed := agg.Flush()
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed, got %d", len(flushed))
	}
	// Count is 2, below minCount of 3, so no suffix
	if strings.Contains(flushed[0].Line.Message, "(x") {
		t.Errorf("should not have aggregation suffix when count < minCount, got %q", flushed[0].Line.Message)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "500ms"},
		{1 * time.Second, "1s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m30s"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestEntryKey(t *testing.T) {
	line := makeLine("source1", "error", "msg1")
	key := entryKey(line)
	if key != "source1|error|msg1" {
		t.Errorf("unexpected key: %q", key)
	}
}

func newTestAggregator(window time.Duration, minCount int) *Aggregator {
	if window <= 0 {
		window = defaultWindow
	}
	if minCount < 2 {
		minCount = defaultMinCount
	}

	a := &Aggregator{
		window:   window,
		minCount: minCount,
		entries:  make(map[string]*entry),
		output:   make(chan []*AggregatedLine, 16),
		done:     make(chan struct{}),
		nowFunc:  time.Now,
	}

	// No ticker goroutine for deterministic tests
	return a
}
