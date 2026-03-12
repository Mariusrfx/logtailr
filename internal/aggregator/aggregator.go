package aggregator

import (
	"fmt"
	"logtailr/pkg/logline"
	"sync"
	"time"
)

const (
	defaultWindow   = 5 * time.Second
	defaultMinCount = 2
)

type AggregatedLine struct {
	Line  *logline.LogLine
	Count int
}

type entry struct {
	line      *logline.LogLine
	count     int
	firstSeen time.Time
	lastSeen  time.Time
}

type Aggregator struct {
	window   time.Duration
	minCount int
	mu       sync.Mutex
	entries  map[string]*entry
	output   chan []*AggregatedLine
	done     chan struct{}
	nowFunc  func() time.Time
}

func New(window time.Duration, minCount int) *Aggregator {
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

	go a.flushLoop()

	return a
}

func (a *Aggregator) Expired() <-chan []*AggregatedLine {
	return a.output
}

func (a *Aggregator) Process(line *logline.LogLine) []*AggregatedLine {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.nowFunc()
	key := entryKey(line)

	var results []*AggregatedLine

	results = append(results, a.collectExpiredLocked(now)...)

	if e, ok := a.entries[key]; ok {
		if now.Sub(e.firstSeen) <= a.window {
			e.count++
			e.lastSeen = now
			return results
		}
		results = append(results, a.emitEntryLocked(key, e))
	}

	a.entries[key] = &entry{
		line:      line,
		count:     1,
		firstSeen: now,
		lastSeen:  now,
	}

	return results
}

func (a *Aggregator) Flush() []*AggregatedLine {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.flushAllLocked()
}

func (a *Aggregator) Stop() {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}

func (a *Aggregator) flushLoop() {
	ticker := time.NewTicker(a.window / 2)
	defer ticker.Stop()

	for {
		select {
		case <-a.done:
			return
		case <-ticker.C:
			a.mu.Lock()
			now := a.nowFunc()
			expired := a.collectExpiredLocked(now)
			a.mu.Unlock()

			if len(expired) > 0 {
				select {
				case a.output <- expired:
				case <-a.done:
					return
				}
			}
		}
	}
}

func (a *Aggregator) collectExpiredLocked(now time.Time) []*AggregatedLine {
	var results []*AggregatedLine
	for key, e := range a.entries {
		if now.Sub(e.firstSeen) > a.window {
			results = append(results, a.emitEntryLocked(key, e))
		}
	}
	return results
}

func (a *Aggregator) emitEntryLocked(key string, e *entry) *AggregatedLine {
	delete(a.entries, key)

	if e.count >= a.minCount {
		duration := e.lastSeen.Sub(e.firstSeen)
		e.line.Message = fmt.Sprintf("%s (x%d in last %s)", e.line.Message, e.count, formatDuration(duration))
	}

	return &AggregatedLine{
		Line:  e.line,
		Count: e.count,
	}
}

func (a *Aggregator) flushAllLocked() []*AggregatedLine {
	if len(a.entries) == 0 {
		return nil
	}

	results := make([]*AggregatedLine, 0, len(a.entries))
	for key, e := range a.entries {
		results = append(results, a.emitEntryLocked(key, e))
	}
	return results
}

func entryKey(line *logline.LogLine) string {
	return line.Source + "|" + line.Level + "|" + line.Message
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	mins := secs / 60
	secs = secs % 60
	if secs == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm%ds", mins, secs)
}
