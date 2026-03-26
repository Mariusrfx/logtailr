package configwatch

import (
	"context"
	"fmt"
	"log"
	"time"

	"logtailr/internal/store"
)

// ChangeType indicates which configuration area changed.
type ChangeType int

const (
	ChangeSources ChangeType = iota
	ChangeOutputs
	ChangeAlertRules
	ChangeSettings
)

// Listener is called when a configuration change is detected.
type Listener func(changeType ChangeType)

// Watcher polls the database for configuration changes by tracking
// the maximum updated_at timestamp of each table.
type Watcher struct {
	store     *store.Store
	interval  time.Duration
	listeners map[ChangeType][]Listener
	watermark map[ChangeType]time.Time
}

// New creates a ConfigWatcher with the given polling interval.
func New(st *store.Store, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Watcher{
		store:     st,
		interval:  interval,
		listeners: make(map[ChangeType][]Listener),
		watermark: make(map[ChangeType]time.Time),
	}
}

// OnChange registers a listener for a specific change type.
func (w *Watcher) OnChange(ct ChangeType, fn Listener) {
	w.listeners[ct] = append(w.listeners[ct], fn)
}

// Start begins polling in a goroutine. Stops when ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	// Initialize watermarks with current timestamps
	w.initWatermarks(ctx)

	go w.run(ctx)
}

func (w *Watcher) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Watcher) initWatermarks(ctx context.Context) {
	tables := map[ChangeType]string{
		ChangeSources:    "sources",
		ChangeOutputs:    "outputs",
		ChangeAlertRules: "alert_rules",
		ChangeSettings:   "settings",
	}

	for ct, table := range tables {
		ts, err := w.maxUpdatedAt(ctx, table)
		if err != nil {
			continue
		}
		w.watermark[ct] = ts
	}
}

func (w *Watcher) poll(ctx context.Context) {
	checks := []struct {
		ct    ChangeType
		table string
	}{
		{ChangeSources, "sources"},
		{ChangeOutputs, "outputs"},
		{ChangeAlertRules, "alert_rules"},
		{ChangeSettings, "settings"},
	}

	for _, c := range checks {
		ts, err := w.maxUpdatedAt(ctx, c.table)
		if err != nil {
			continue
		}

		prev, ok := w.watermark[c.ct]
		if !ok || ts.After(prev) {
			w.watermark[c.ct] = ts
			if ok { // Don't fire on first read
				w.notify(c.ct)
			}
		}
	}
}

func (w *Watcher) notify(ct ChangeType) {
	for _, fn := range w.listeners[ct] {
		fn(ct)
	}
}

func (w *Watcher) maxUpdatedAt(ctx context.Context, table string) (time.Time, error) {
	// Validate table name to prevent SQL injection
	validTables := map[string]bool{
		"sources":     true,
		"outputs":     true,
		"alert_rules": true,
		"settings":    true,
	}
	if !validTables[table] {
		return time.Time{}, fmt.Errorf("invalid table name: %s", table)
	}

	var ts time.Time
	//nolint:gosec // table name is validated above against a whitelist
	query := fmt.Sprintf(`SELECT COALESCE(MAX(updated_at), '1970-01-01'::timestamptz) FROM %s`, table)
	err := w.store.Pool.QueryRow(ctx, query).Scan(&ts)
	if err != nil {
		log.Printf("configwatch: error polling %s: %v", table, err)
		return time.Time{}, err
	}
	return ts, nil
}
