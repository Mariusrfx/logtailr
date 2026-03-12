package alert

import (
	"fmt"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	maxRecentEvents  = 100
	processQueueSize = 512
)

type evaluator interface {
	evaluate(line *logline.LogLine) *Event
	rule() *Rule
}

type Engine struct {
	evaluators []evaluator
	notifiers  []Notifier
	limiter    *rateLimiter

	recentMu sync.RWMutex
	recent   []*Event

	ruleMu    sync.RWMutex
	ruleStats map[string]*ruleState

	processCh chan *logline.LogLine
	done      chan struct{}
}

type ruleState struct {
	FireCount int       `json:"fire_count"`
	LastFired time.Time `json:"last_fired,omitzero"`
}

func NewEngine(rules []Rule, notifiers []Notifier) (*Engine, error) {
	evals := make([]evaluator, 0, len(rules))
	stats := make(map[string]*ruleState, len(rules))

	for i := range rules {
		ev, err := compileRule(&rules[i])
		if err != nil {
			return nil, fmt.Errorf("alert rule %q: %w", rules[i].Name, err)
		}
		evals = append(evals, ev)
		stats[rules[i].Name] = &ruleState{}
	}

	e := &Engine{
		evaluators: evals,
		notifiers:  notifiers,
		limiter:    newRateLimiter(),
		recent:     make([]*Event, 0, maxRecentEvents),
		ruleStats:  stats,
		processCh:  make(chan *logline.LogLine, processQueueSize),
		done:       make(chan struct{}),
	}

	go e.processLoop()

	return e, nil
}

func (e *Engine) ProcessLine(line *logline.LogLine) {
	select {
	case e.processCh <- line:
	default:
	}
}

func (e *Engine) ProcessHealthChange(source string, oldStatus, newStatus health.Status) {
	for _, ev := range e.evaluators {
		r := ev.rule()
		if r.Type != RuleTypeHealthChange {
			continue
		}
		if r.Source != "" && r.Source != source {
			continue
		}

		event := &Event{
			Rule:      r.Name,
			Severity:  string(r.Severity),
			Message:   fmt.Sprintf("Source %q health changed: %s -> %s", source, oldStatus, newStatus),
			Source:    source,
			Timestamp: time.Now(),
		}

		e.fireEvent(r, event)
	}
}

func (e *Engine) RecentEvents(limit int) []*Event {
	e.recentMu.RLock()
	defer e.recentMu.RUnlock()

	if limit <= 0 || limit > len(e.recent) {
		limit = len(e.recent)
	}

	start := len(e.recent) - limit
	result := make([]*Event, limit)
	for i, j := start, 0; i < len(e.recent); i, j = i+1, j+1 {
		result[j] = e.recent[i]
	}

	return result
}

func (e *Engine) RuleStats() map[string]*ruleState {
	e.ruleMu.RLock()
	defer e.ruleMu.RUnlock()

	result := make(map[string]*ruleState, len(e.ruleStats))
	for k, v := range e.ruleStats {
		copied := *v
		result[k] = &copied
	}
	return result
}

func (e *Engine) Rules() []Rule {
	rules := make([]Rule, len(e.evaluators))
	for i, ev := range e.evaluators {
		rules[i] = *ev.rule()
	}
	return rules
}

func (e *Engine) Close() error {
	close(e.processCh)
	<-e.done

	var firstErr error
	for _, n := range e.notifiers {
		if err := n.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (e *Engine) processLoop() {
	defer close(e.done)

	for line := range e.processCh {
		for _, ev := range e.evaluators {
			if event := ev.evaluate(line); event != nil {
				e.fireEvent(ev.rule(), event)
			}
		}
	}
}

func (e *Engine) fireEvent(r *Rule, event *Event) {
	if !e.limiter.Allow(r.Name, r.Cooldown) {
		return
	}

	e.ruleMu.Lock()
	if st, ok := e.ruleStats[r.Name]; ok {
		st.FireCount++
		st.LastFired = event.Timestamp
	}
	e.ruleMu.Unlock()

	e.recentMu.Lock()
	if len(e.recent) >= maxRecentEvents {
		e.recent = e.recent[1:]
	}
	e.recent = append(e.recent, event)
	e.recentMu.Unlock()

	for _, n := range e.notifiers {
		if err := n.Notify(event); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "alert notify error: %v\n", err)
		}
	}
}

type patternEvaluator struct {
	r       *Rule
	pattern *regexp.Regexp
}

func (pe *patternEvaluator) evaluate(line *logline.LogLine) *Event {
	if pe.r.Source != "" && pe.r.Source != line.Source {
		return nil
	}
	if !pe.pattern.MatchString(line.Message) {
		return nil
	}
	return &Event{
		Rule:      pe.r.Name,
		Severity:  string(pe.r.Severity),
		Message:   fmt.Sprintf("Pattern matched: %s", line.Message),
		Source:    line.Source,
		Timestamp: time.Now(),
	}
}

func (pe *patternEvaluator) rule() *Rule { return pe.r }

type levelEvaluator struct {
	r        *Rule
	minLevel int
}

func (le *levelEvaluator) evaluate(line *logline.LogLine) *Event {
	if le.r.Source != "" && le.r.Source != line.Source {
		return nil
	}
	lineLevel, ok := logline.LogLevels[strings.ToLower(line.Level)]
	if !ok {
		return nil
	}
	if lineLevel < le.minLevel {
		return nil
	}
	return &Event{
		Rule:      le.r.Name,
		Severity:  string(le.r.Severity),
		Message:   fmt.Sprintf("[%s] %s", strings.ToUpper(line.Level), line.Message),
		Source:    line.Source,
		Timestamp: time.Now(),
	}
}

func (le *levelEvaluator) rule() *Rule { return le.r }

type errorRateEvaluator struct {
	r          *Rule
	mu         sync.Mutex
	timestamps []time.Time
}

func (er *errorRateEvaluator) evaluate(line *logline.LogLine) *Event {
	if er.r.Source != "" && er.r.Source != line.Source {
		return nil
	}

	lineLevel, ok := logline.LogLevels[strings.ToLower(line.Level)]
	if !ok {
		return nil
	}
	if lineLevel < logline.LogLevels["error"] {
		return nil
	}

	now := time.Now()
	er.mu.Lock()
	defer er.mu.Unlock()

	er.timestamps = append(er.timestamps, now)

	cutoff := now.Add(-er.r.Window)
	start := 0
	for start < len(er.timestamps) && er.timestamps[start].Before(cutoff) {
		start++
	}
	er.timestamps = er.timestamps[start:]

	count := len(er.timestamps)
	if count >= er.r.Threshold {
		er.timestamps = er.timestamps[:0]
		return &Event{
			Rule:      er.r.Name,
			Severity:  string(er.r.Severity),
			Message:   fmt.Sprintf("Error rate exceeded: %d errors in %s", count, er.r.Window),
			Source:    line.Source,
			Timestamp: now,
			Count:     count,
		}
	}

	return nil
}

func (er *errorRateEvaluator) rule() *Rule { return er.r }

type healthChangeEvaluator struct {
	r *Rule
}

func (he *healthChangeEvaluator) evaluate(_ *logline.LogLine) *Event {
	return nil
}

func (he *healthChangeEvaluator) rule() *Rule { return he.r }

func compileRule(r *Rule) (evaluator, error) {
	switch r.Type {
	case RuleTypePattern:
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
		return &patternEvaluator{r: r, pattern: re}, nil

	case RuleTypeLevel:
		lvl, ok := logline.LogLevels[strings.ToLower(r.Level)]
		if !ok {
			return nil, fmt.Errorf("invalid level %q", r.Level)
		}
		return &levelEvaluator{r: r, minLevel: lvl}, nil

	case RuleTypeErrorRate:
		return &errorRateEvaluator{r: r}, nil

	case RuleTypeHealthChange:
		return &healthChangeEvaluator{r: r}, nil

	default:
		return nil, fmt.Errorf("unknown rule type %q", r.Type)
	}
}
