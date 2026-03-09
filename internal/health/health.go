package health

import (
	"fmt"
	"sync"
	"time"
)

// Status represents the health status of a source
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

// SourceHealth contains the health state of a specific source
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

// Monitor manages the health status of all sources
type Monitor struct {
	mu      sync.RWMutex
	sources map[string]*SourceHealth
}

// NewMonitor creates a new health monitor
func NewMonitor() *Monitor {
	return &Monitor{
		sources: make(map[string]*SourceHealth),
	}
}

// RegisterSource registers a new source for monitoring
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

// UpdateStatus updates the status of a source
func (m *Monitor) UpdateStatus(name string, status Status, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	source := m.getOrCreateSource(name)
	source.Status = status
	source.LastUpdate = time.Now()

	if err != nil {
		source.LastError = err
		source.ErrorCount++
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

// MarkHealthy marks a source as healthy
func (m *Monitor) MarkHealthy(name string) {
	m.UpdateStatus(name, StatusHealthy, nil)
}

// MarkFailed marks a source as failed
func (m *Monitor) MarkFailed(name string, err error) {
	m.UpdateStatus(name, StatusFailed, err)
}

// MarkDegraded marks a source as degraded
func (m *Monitor) MarkDegraded(name string, err error) {
	m.UpdateStatus(name, StatusDegraded, err)
}

// MarkStopped marks a source as stopped
func (m *Monitor) MarkStopped(name string) {
	m.UpdateStatus(name, StatusStopped, nil)
}

// GetStatus returns the status of a specific source
func (m *Monitor) GetStatus(name string) (*SourceHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	source, exists := m.sources[name]
	if !exists {
		return nil, false
	}

	return source.copy(), true
}

// GetAllStatuses returns the status of all sources
func (m *Monitor) GetAllStatuses() map[string]*SourceHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]*SourceHealth, len(m.sources))
	for name, source := range m.sources {
		statuses[name] = source.copy()
	}

	return statuses
}

// GetHealthySources returns only the healthy sources
func (m *Monitor) GetHealthySources() []*SourceHealth {
	return m.getSourcesByStatus(StatusHealthy)
}

// GetFailedSources returns only the failed sources
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

// GetHealthCount returns the count of sources by status
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

// IsAllHealthy returns true if all sources are healthy
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

// Summary generates a summary of the health status
func (m *Monitor) Summary() string {
	healthy, degraded, failed, stopped := m.GetHealthCount()
	total := len(m.sources)

	return fmt.Sprintf("Sources: %d total | ✓ %d healthy | ⚠ %d degraded | ✗ %d failed | ⏸ %d stopped",
		total, healthy, degraded, failed, stopped)
}

// String returns a string representation of the status
func (s Status) String() string {
	return string(s)
}

// Symbol returns the visual symbol for the status
func (s Status) Symbol() string {
	if symbol, ok := statusSymbols[s]; ok {
		return symbol
	}
	return "?"
}
