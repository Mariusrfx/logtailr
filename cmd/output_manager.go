package cmd

import (
	"log"
	"sync"

	"logtailr/internal/config"
	"logtailr/internal/output"
	"logtailr/pkg/logline"
)

// OutputManager manages a dynamic output writer that can be swapped at runtime.
type OutputManager struct {
	mu     sync.RWMutex
	writer output.Writer
}

// NewOutputManager creates a manager with an initial writer.
func NewOutputManager(w output.Writer) *OutputManager {
	return &OutputManager{writer: w}
}

// Write delegates to the current writer under a read lock.
func (om *OutputManager) Write(line *logline.LogLine) error {
	om.mu.RLock()
	w := om.writer
	om.mu.RUnlock()
	return w.Write(line)
}

// Close closes the current writer.
func (om *OutputManager) Close() error {
	om.mu.RLock()
	w := om.writer
	om.mu.RUnlock()
	return w.Close()
}

// Swap replaces the current writer with a new one built from the given config.
// The old writer is closed after the swap.
func (om *OutputManager) Swap(outputsCfg *config.OutputsConfig) {
	newWriter, err := createWriter(outputsCfg)
	if err != nil {
		log.Printf("Hot-reload: failed to create new output writer: %v", err)
		return
	}

	om.mu.Lock()
	old := om.writer
	om.writer = newWriter
	om.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			log.Printf("Hot-reload: error closing old writer: %v", err)
		}
	}

	log.Println("Hot-reload: output writer swapped successfully")
}
