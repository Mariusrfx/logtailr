package cmd

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"logtailr/internal/config"
	"logtailr/pkg/logline"
)

// recordingWriter records written lines and reports Close calls.
type recordingWriter struct {
	mu     sync.Mutex
	lines  []*logline.LogLine
	closed atomic.Bool
}

func (r *recordingWriter) Write(line *logline.LogLine) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
	return nil
}

func (r *recordingWriter) Close() error {
	r.closed.Store(true)
	return nil
}

func (r *recordingWriter) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.lines)
}

// blockingWriter blocks its first Write until release is closed, simulating a
// writer whose network calls are stuck.
type blockingWriter struct {
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	lines   []*logline.LogLine
	closed  atomic.Bool
}

func (b *blockingWriter) Write(line *logline.LogLine) error {
	b.once.Do(func() { <-b.release })
	b.mu.Lock()
	b.lines = append(b.lines, line)
	b.mu.Unlock()
	return nil
}

func (b *blockingWriter) Close() error {
	b.closed.Store(true)
	return nil
}

func mkTestLine(n int) *logline.LogLine {
	return &logline.LogLine{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "line",
		Source:    "test",
		Fields:    map[string]interface{}{"n": n},
	}
}

func waitFor(t *testing.T, cond func() bool, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestOutputManager_WritesLinesAsynchronously(t *testing.T) {
	rec := &recordingWriter{}
	om := NewOutputManager(rec)

	for i := 0; i < 10; i++ {
		if err := om.Write(mkTestLine(i)); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
	}

	waitFor(t, func() bool { return rec.count() == 10 }, 5*time.Second)
	if err := om.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !rec.closed.Load() {
		t.Fatal("underlying writer was not closed")
	}
	if rec.count() != 10 {
		t.Fatalf("expected 10 lines, got %d", rec.count())
	}
}

func TestOutputManager_DropsWhenQueueFull(t *testing.T) {
	bw := &blockingWriter{release: make(chan struct{})}
	om := NewOutputManager(bw)
	defer close(bw.release) // unblock before Close so the drain finishes

	// The writer goroutine takes one line and blocks in Write, so the queue
	// absorbs the next 4096 lines; everything after must be dropped without
	// ever blocking the caller. Depending on whether the writer has already
	// consumed the first line, drops are 100 or 101.
	total := outputQueueBuffer + 101
	for i := 0; i < total; i++ {
		om.Write(mkTestLine(i))
	}

	if got := om.Dropped(); got < 100 {
		t.Fatalf("expected at least 100 drops with a blocked writer, got %d", got)
	}
}

func TestOutputManager_CloseDrainsQueue(t *testing.T) {
	rec := &recordingWriter{}
	om := NewOutputManager(rec)

	for i := 0; i < 50; i++ {
		om.Write(mkTestLine(i))
	}

	if err := om.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := rec.count(); got != 50 {
		t.Fatalf("expected all 50 queued lines after Close, got %d", got)
	}
}

func TestOutputManager_SwapClosesOldWriter(t *testing.T) {
	old := &recordingWriter{}
	om := NewOutputManager(old)

	om.Write(mkTestLine(0))
	waitFor(t, func() bool { return old.count() == 1 }, 5*time.Second)

	// Swap with an empty outputs config: the primary writer is a console
	// writer (outputFlag defaults to ""), no extra outputs are configured.
	om.Swap(&config.OutputsConfig{})

	waitFor(t, func() bool { return old.closed.Load() }, 5*time.Second)
}
