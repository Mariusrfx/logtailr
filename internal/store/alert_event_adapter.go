package store

import (
	"context"
	"time"

	"logtailr/internal/alert"
)

// AlertEventAdapter adapts Store to satisfy the alert.EventStore interface.
type AlertEventAdapter struct {
	store *Store
}

// NewAlertEventAdapter creates an adapter that implements alert.EventStore.
func NewAlertEventAdapter(s *Store) *AlertEventAdapter {
	return &AlertEventAdapter{store: s}
}

// CreateAlertEvent persists a fired alert event to the database.
func (a *AlertEventAdapter) CreateAlertEvent(ctx context.Context, e *alert.StoredEvent) error {
	row := &AlertEventRow{
		RuleName: e.RuleName,
		Severity: e.Severity,
		Message:  e.Message,
		Source:   e.Source,
		Count:    e.Count,
		FiredAt:  e.FiredAt,
	}
	return a.store.CreateAlertEvent(ctx, row)
}

// DeleteAlertEventsOlderThan removes events older than the given time.
func (a *AlertEventAdapter) DeleteAlertEventsOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return a.store.DeleteAlertEventsOlderThan(ctx, before)
}
