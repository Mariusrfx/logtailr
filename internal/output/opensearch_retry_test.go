package output

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func waitForCond(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func bulkDocMessages(t *testing.T, body []byte) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	msgs := make([]string, 0, len(lines)/2)
	for i := 1; i < len(lines); i += 2 {
		var doc map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &doc); err != nil {
			t.Fatalf("doc line not valid JSON: %v", err)
		}
		msgs = append(msgs, fmt.Sprint(doc["message"]))
	}
	return msgs
}

func TestOpenSearchWriter_PendingResendAfterRecovery(t *testing.T) {
	var attempts, received atomic.Int32
	failing := atomic.Bool{}
	failing.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		attempts.Add(1)
		if failing.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		body, _ := io.ReadAll(r.Body)
		received.Add(int32(len(bulkDocMessages(t, body))))
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:         []string{server.URL},
		Index:         "test",
		BulkSize:      3,
		FlushInterval: "200ms",
		MaxRetries:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		// Only the buffer-filling write can return the flush error; the docs
		// are kept in the retry queue either way.
		_ = ow.Write(newTestLine("info", fmt.Sprintf("m%d", i)))
	}

	// Total failure must queue the docs, not lose them.
	waitForCond(t, 3*time.Second, func() bool { return ow.PendingDocs() == 3 })

	failing.Store(false)
	waitForCond(t, 5*time.Second, func() bool { return received.Load() == 3 })

	if err := ow.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if ow.PendingDocs() != 0 {
		t.Fatalf("pending after recovery = %d, want 0", ow.PendingDocs())
	}
}

func TestOpenSearchWriter_PartialFailureRetriesOnlyFailed(t *testing.T) {
	var mu sync.Mutex
	accepted := map[string]int{}
	sentTimes := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		msgs := bulkDocMessages(t, body)

		mu.Lock()
		for _, m := range msgs {
			sentTimes[m]++
		}
		var items []map[string]any
		anyErr := false
		for _, m := range msgs {
			// Reject "bad" on its first attempt only, so the retry is accepted.
			if m == "bad" && sentTimes[m] == 1 {
				items = append(items, map[string]any{"index": map[string]any{"status": 400}})
				anyErr = true
				continue
			}
			accepted[m]++
			items = append(items, map[string]any{"index": map[string]any{"status": 201}})
		}
		mu.Unlock()

		resp := map[string]any{"errors": anyErr, "items": items}
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:      []string{server.URL},
		Index:      "test",
		BulkSize:   3,
		MaxRetries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Only the buffer-filling write can return the flush error; the docs are
	// kept either way.
	for _, msg := range []string{"good1", "bad", "good2"} {
		_ = ow.Write(newTestLine("info", msg))
	}

	if err := ow.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if accepted["good1"] != 1 || accepted["good2"] != 1 {
		t.Fatalf("good docs must be accepted exactly once: %v", accepted)
	}
	if accepted["bad"] != 1 {
		t.Fatalf("bad doc must end accepted once: %v", accepted)
	}
	if sentTimes["good1"] != 1 || sentTimes["good2"] != 1 {
		t.Fatalf("good docs must not be re-sent: %v", sentTimes)
	}
	if sentTimes["bad"] != 2 {
		t.Fatalf("bad doc must be re-sent once: %v", sentTimes)
	}
	if ow.PendingDocs() != 0 {
		t.Fatalf("pending = %d, want 0", ow.PendingDocs())
	}
}

func TestOpenSearchWriter_PendingCapDropsOldest(t *testing.T) {
	oldCap := maxPendingDocs
	maxPendingDocs = 10
	defer func() { maxPendingDocs = oldCap }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:      []string{server.URL},
		Index:      "test",
		BulkSize:   2,
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Flushes fail while the server is down; docs accumulate in the queue.
	for i := range 24 {
		_ = ow.Write(newTestLine("info", fmt.Sprintf("m%d", i)))
	}

	if got := ow.PendingDocs(); got != 10 {
		t.Fatalf("pending = %d, want the cap (10)", got)
	}
	if got := ow.PendingDropped(); got != 14 {
		t.Fatalf("dropped = %d, want 14", got)
	}

	if err := ow.Close(); err == nil {
		t.Fatal("Close with unreachable server must report the undelivered docs")
	}
}

func TestOpenSearchWriter_CloseDeliversPendingAfterRecovery(t *testing.T) {
	var received atomic.Int32
	failing := atomic.Bool{}
	failing.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		if failing.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		body, _ := io.ReadAll(r.Body)
		received.Add(int32(len(bulkDocMessages(t, body))))
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:      []string{server.URL},
		Index:      "test",
		BulkSize:   1,
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := ow.Write(newTestLine("info", "only")); err == nil {
		t.Fatal("expected the flush error, docs go to the retry queue")
	}
	waitForCond(t, 3*time.Second, func() bool { return ow.PendingDocs() == 1 })

	failing.Store(false)
	if err := ow.Close(); err != nil {
		t.Fatalf("Close after recovery: %v", err)
	}
	if received.Load() != 1 {
		t.Fatalf("received = %d, want 1", received.Load())
	}
}

func TestOpenSearchWriter_CloseWithServerDownCountsLoss(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:      []string{server.URL},
		Index:      "test",
		BulkSize:   1,
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := ow.Write(newTestLine("info", "lost")); err == nil {
		t.Fatal("expected the flush error, docs go to the retry queue")
	}
	waitForCond(t, 3*time.Second, func() bool { return ow.PendingDocs() == 1 })

	err = ow.Close()
	if err == nil || !strings.Contains(err.Error(), "not delivered") {
		t.Fatalf("expected a 'not delivered' error, got %v", err)
	}
	if got := ow.PendingDropped(); got != 1 {
		t.Fatalf("PendingDropped = %d, want 1", got)
	}
}

type stats struct {
	failed  int64
	pending int
	dropped int64
}

func TestOpenSearchWriter_StatsSink(t *testing.T) {
	statsCh := make(chan stats, 16)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:      []string{server.URL},
		Index:      "test",
		BulkSize:   1,
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	ow.SetStatsSink(func(failed int64, pending int, dropped int64) {
		statsCh <- stats{failed: failed, pending: pending, dropped: dropped}
	})

	if err := ow.Write(newTestLine("info", "x")); err == nil {
		t.Fatal("expected the flush error, docs go to the retry queue")
	}

	// Two emissions: one when the batch is counted, one when the doc is queued.
	first := recvStats(t, statsCh)
	last := recvStats(t, statsCh)
	if last.failed < 1 || last.pending != 1 || last.dropped != 0 {
		t.Fatalf("unexpected final stats: first=%+v last=%+v", first, last)
	}
}

func recvStats(t *testing.T, ch chan stats) stats {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(3 * time.Second):
		t.Fatal("stats sink was not called")
		return stats{}
	}
}
