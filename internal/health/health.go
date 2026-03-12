package health

import (
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
)

var statusSymbols = map[Status]string{
	StatusHealthy:  "✓",
	StatusDegraded: "⚠",
	StatusFailed:   "✗",
	StatusStopped:  "⏸",
	StatusStarting: "⏳",
}

type SourceHealth struct {
	Name       string
	Status     Status
	LastError  error
	LastUpdate time.Time
	ErrorCount int
	StartTime  time.Time
}

func (s *SourceHealth) copy() *SourceHealth {
	copied := *s
	return &copied
}

type StatusChangeFunc func(source string, oldStatus, newStatus Status)

type Monitor struct {
	mu       sync.RWMutex
	sources  map[string]*SourceHealth
	onChange StatusChangeFunc
}

func NewMonitor() *Monitor {
	return &Monitor{
		sources: make(map[string]*SourceHealth),
	}
}

func (m *Monitor) RegisterSource(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.sources[name] = &SourceHealth{
		Name:       name,
		Status:     StatusStarting,
		StartTime:  now,
		LastUpdate: now,
	}
}

func (m *Monitor) SetOnChange(fn StatusChangeFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

func (m *Monitor) UpdateStatus(name string, status Status, err error) {
	m.mu.Lock()

	source := m.getOrCreateSource(name)
	oldStatus := source.Status
	source.Status = status
	source.LastUpdate = time.Now()

	if err != nil {
		source.LastError = err
		source.ErrorCount++
	}

	onChange := m.onChange
	m.mu.Unlock()

	if onChange != nil && oldStatus != status {
		onChange(name, oldStatus, status)
	}
}

func (m *Monitor) getOrCreateSource(name string) *SourceHealth {
	source, exists := m.sources[name]
	if !exists {
		source = &SourceHealth{
			Name:      name,
			StartTime: time.Now(),
		}
		m.sources[name] = source
	}
	return source
}

func (m *Monitor) MarkHealthy(name string) {
	m.UpdateStatus(name, StatusHealthy, nil)
}

func (m *Monitor) MarkFailed(name string, err error) {
	m.UpdateStatus(name, StatusFailed, err)
}

func (m *Monitor) MarkDegraded(name string, err error) {
	m.UpdateStatus(name, StatusDegraded, err)
}

func (m *Monitor) MarkStopped(name string) {
	m.UpdateStatus(name, StatusStopped, nil)
}

func (m *Monitor) GetStatus(name string) (*SourceHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	source, exists := m.sources[name]
	if !exists {
		return nil, false
	}

	return source.copy(), true
}

func (m *Monitor) GetAllStatuses() map[string]*SourceHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]*SourceHealth, len(m.sources))
	for name, source := range m.sources {
		statuses[name] = source.copy()
	}

	return statuses
}

func (m *Monitor) GetHealthySources() []*SourceHealth {
	return m.getSourcesByStatus(StatusHealthy)
}

func (m *Monitor) GetFailedSources() []*SourceHealth {
	return m.getSourcesByStatus(StatusFailed)
}

func (m *Monitor) getSourcesByStatus(status Status) []*SourceHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SourceHealth
	for _, source := range m.sources {
		if source.Status == status {
			result = append(result, source.copy())
		}
	}

	return result
}

func (m *Monitor) GetHealthCount() (healthy, degraded, failed, stopped int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, source := range m.sources {
		switch source.Status {
		case StatusHealthy:
			healthy++
		case StatusDegraded:
			degraded++
		case StatusFailed:
			failed++
		case StatusStopped:
			stopped++
		}
	}

	return
}

func (m *Monitor) IsAllHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, source := range m.sources {
		if source.Status != StatusHealthy {
			return false
		}
	}

	return true
}

func (m *Monitor) Summary() string {
	healthy, degraded, failed, stopped := m.GetHealthCount()
	total := len(m.sources)

	return fmt.Sprintf("Sources: %d total | ✓ %d healthy | ⚠ %d degraded | ✗ %d failed | ⏸ %d stopped",
		total, healthy, degraded, failed, stopped)
}

func (s Status) String() string {
	return string(s)
}

func (s Status) Symbol() string {
	if symbol, ok := statusSymbols[s]; ok {
		return symbol
	}
	return "?"
}
