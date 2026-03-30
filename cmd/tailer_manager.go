package cmd

import (
	"context"
	"fmt"
	"log"
	"sync"

	"logtailr/internal/health"
	"logtailr/internal/tailer"
	"logtailr/pkg/logline"
)

// TailerManager manages a dynamic set of tailers that can be added, removed,
// or restarted at runtime in response to configuration changes.
type TailerManager struct {
	mu      sync.RWMutex
	tailers map[string]tailer.Tailer // keyed by source name
	sources map[string]logline.SourceConfig
	monitor *health.Monitor
	logChan chan<- *logline.LogLine
	errChan chan<- error
	ctx     context.Context
}

// NewTailerManager creates a manager with the given channels and health monitor.
func NewTailerManager(ctx context.Context, monitor *health.Monitor, logChan chan<- *logline.LogLine, errChan chan<- error) *TailerManager {
	return &TailerManager{
		tailers: make(map[string]tailer.Tailer),
		sources: make(map[string]logline.SourceConfig),
		monitor: monitor,
		logChan: logChan,
		errChan: errChan,
		ctx:     ctx,
	}
}

// Add creates and starts a tailer for the given source.
func (tm *TailerManager) Add(src logline.SourceConfig) error {
	t, err := createTailer(src, tm.monitor)
	if err != nil {
		return fmt.Errorf("source %q: %w", src.Name, err)
	}

	tm.mu.Lock()
	tm.tailers[src.Name] = t
	tm.sources[src.Name] = src
	tm.mu.Unlock()

	t.Start(tm.ctx, tm.logChan, tm.errChan)
	return nil
}

// Remove stops and removes the tailer for the named source.
func (tm *TailerManager) Remove(name string) {
	tm.mu.Lock()
	t, ok := tm.tailers[name]
	if ok {
		delete(tm.tailers, name)
		delete(tm.sources, name)
	}
	tm.mu.Unlock()

	if ok {
		_ = t.Stop()
	}
}

// StopAll stops all managed tailers.
func (tm *TailerManager) StopAll() {
	tm.mu.Lock()
	tailers := make(map[string]tailer.Tailer, len(tm.tailers))
	for k, v := range tm.tailers {
		tailers[k] = v
	}
	tm.tailers = make(map[string]tailer.Tailer)
	tm.sources = make(map[string]logline.SourceConfig)
	tm.mu.Unlock()

	for _, t := range tailers {
		_ = t.Stop()
	}
}

// Reconcile compares the current set of tailers with a new set of source configs
// and adds/removes/restarts tailers as needed.
func (tm *TailerManager) Reconcile(newSources []logline.SourceConfig) {
	newMap := make(map[string]logline.SourceConfig, len(newSources))
	for _, s := range newSources {
		newMap[s.Name] = s
	}

	tm.mu.RLock()
	currentNames := make(map[string]bool, len(tm.sources))
	currentConfigs := make(map[string]logline.SourceConfig, len(tm.sources))
	for name, src := range tm.sources {
		currentNames[name] = true
		currentConfigs[name] = src
	}
	tm.mu.RUnlock()

	// Remove tailers no longer in config
	for name := range currentNames {
		if _, exists := newMap[name]; !exists {
			log.Printf("Hot-reload: removing source %q", name)
			tm.Remove(name)
		}
	}

	// Add new tailers or restart changed ones
	for name, newSrc := range newMap {
		if !currentNames[name] {
			log.Printf("Hot-reload: adding source %q", name)
			if err := tm.Add(newSrc); err != nil {
				log.Printf("Hot-reload: failed to add source %q: %v", name, err)
			}
			continue
		}
		if sourceChanged(currentConfigs[name], newSrc) {
			log.Printf("Hot-reload: restarting source %q (config changed)", name)
			tm.Remove(name)
			if err := tm.Add(newSrc); err != nil {
				log.Printf("Hot-reload: failed to restart source %q: %v", name, err)
			}
		}
	}
}

// Get returns the tailer for the named source, or nil.
func (tm *TailerManager) Get(name string) tailer.Tailer {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tailers[name]
}

// sourceChanged compares two source configs to determine if the tailer needs restart.
func sourceChanged(old, new logline.SourceConfig) bool {
	return old.Type != new.Type ||
		old.Path != new.Path ||
		old.Container != new.Container ||
		old.Unit != new.Unit ||
		old.Follow != new.Follow ||
		old.Parser != new.Parser ||
		old.Namespace != new.Namespace ||
		old.Pod != new.Pod ||
		old.LabelSelector != new.LabelSelector ||
		old.Kubeconfig != new.Kubeconfig ||
		old.Priority != new.Priority ||
		old.OutputFormat != new.OutputFormat
}
