package alert

import (
	"context"
	"time"
)

// EventStore is an optional interface for persisting alert events to a database.
type EventStore interface {
	CreateAlertEvent(ctx context.Context, e *StoredEvent) error
	DeleteAlertEventsOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// StoredEvent is the struct passed to EventStore for persistence.
type StoredEvent struct {
	RuleName string
	Severity string
	Message  string
	Source   string
	Count    int
	FiredAt  time.Time
}

type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type RuleType string

const (
	RuleTypePattern      RuleType = "pattern"
	RuleTypeLevel        RuleType = "level"
	RuleTypeHealthChange RuleType = "health_change"
	RuleTypeErrorRate    RuleType = "error_rate"
)

type Rule struct {
	Name      string
	Type      RuleType
	Severity  Severity
	Pattern   string
	Level     string
	Source    string
	Threshold int
	Window    time.Duration
	Cooldown  time.Duration
}

type Event struct {
	Rule      string    `json:"rule"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count,omitempty"`
}
