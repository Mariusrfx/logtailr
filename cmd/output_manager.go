package cmd

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"logtailr/internal/config"
	"logtailr/internal/output"
	"logtailr/internal/safego"
	"logtailr/pkg/logline"
)

// outputQueueBuffer bounds the number of lines waiting to be written. When it
// is full, new lines are dropped (and counted) instead of back-pressuring the
// pipeline.
const outputQueueBuffer = 4096

// outEvent is one item in the output queue: either a log line to write or a
// hot-reload swap request. Processing both in the same ordered queue guarantees
// that a swap (and the closing of the old writer) only happens after every
// queued line has been written, and that no line is ever written to a writer
// that is being closed.
type outEvent struct {
	line *logline.LogLine
	swap *config.OutputsConfig
}

// OutputManager manages a dynamic output writer that can be swapped at
// runtime. Lines are enqueued non-blocking and written by a single goroutine,
// so a slow or blocking writer (e.g. OpenSearch down) can never stall the log
// pipeline: it only causes drops, which are observable via Dropped()/OnDrop.
type OutputManager struct {
	mu     sync.RWMutex
	writer output.Writer

	queue   chan outEvent
	dropped atomic.Int64
	onDrop  func(count int64)

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // closed when the writer loop has drained and exited
}

// NewOutputManager creates a manager with an initial writer and starts its
// writer goroutine.
func NewOutputManager(w output.Writer) *OutputManager {
	ctx, cancel := context.WithCancel(context.Background())
	om := &OutputManager{
		writer: w,
		queue:  make(chan outEvent, outputQueueBuffer),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	safego.Go("output-writer", om.writerLoop, nil)
	return om
}

// OnDrop registers a callback invoked with the cumulative drop count each
// time a line is dropped because the queue is full.
func (om *OutputManager) OnDrop(fn func(count int64)) {
	om.onDrop = fn
}

// Dropped returns the total number of lines dropped because the queue was full.
func (om *OutputManager) Dropped() int64 {
	return om.dropped.Load()
}

func (om *OutputManager) Writer() output.Writer {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.writer
}

// Write enqueues a line without blocking. It never fails and never blocks, so
// the pipeline is protected from slow writers.
func (om *OutputManager) Write(line *logline.LogLine) error {
	select {
	case om.queue <- outEvent{line: line}:
	default:
		om.drop()
	}
	return nil
}

func (om *OutputManager) drop() {
	n := om.dropped.Add(1)
	if fn := om.onDrop; fn != nil {
		fn(n)
	}
	if n == 1 || n%1024 == 0 {
		slog.Warn("output queue full, dropping log lines", "total_dropped", n)
	}
}

// Swap enqueues a hot-reload request. The swap itself (creating the new
// writer and closing the old one) runs in the writer goroutine, so it is
// ordered after all previously queued lines.
func (om *OutputManager) Swap(outputsCfg *config.OutputsConfig) {
	select {
	case om.queue <- outEvent{swap: outputsCfg}:
	case <-om.ctx.Done():
	}
}

// Close stops the manager: the writer goroutine drains the remaining queue
// (an in-flight write is bounded by the writer's own timeouts) and then the
// current writer is closed, which triggers its final flush.
func (om *OutputManager) Close() error {
	om.cancel()
	<-om.done

	om.mu.RLock()
	w := om.writer
	om.mu.RUnlock()
	if w == nil {
		return nil
	}
	return w.Close()
}

func (om *OutputManager) writerLoop() {
	defer close(om.done)

	for {
		select {
		case <-om.ctx.Done():
			om.drainQueue()
			return
		case ev := <-om.queue:
			if ev.swap != nil {
				om.performSwap(ev.swap)
				continue
			}
			om.writeOne(ev.line)
		}
	}
}

// drainQueue writes whatever is still queued once shutdown has been signalled.
func (om *OutputManager) drainQueue() {
	for {
		select {
		case ev := <-om.queue:
			if ev.swap != nil {
				om.performSwap(ev.swap)
				continue
			}
			om.writeOne(ev.line)
		default:
			return
		}
	}
}

func (om *OutputManager) writeOne(line *logline.LogLine) {
	om.mu.RLock()
	w := om.writer
	om.mu.RUnlock()
	if err := w.Write(line); err != nil {
		slog.Error("output write failed", "error", err)
	}
}

func (om *OutputManager) performSwap(outputsCfg *config.OutputsConfig) {
	newWriter, err := createWriter(outputsCfg)
	if err != nil {
		slog.Error("hot-reload: failed to create new output writer", "error", err)
		return
	}

	om.mu.Lock()
	old := om.writer
	om.writer = newWriter
	om.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			slog.Error("hot-reload: error closing old writer", "error", err)
		}
	}

	slog.Info("hot-reload: output writer swapped")
}
