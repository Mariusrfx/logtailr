package alert

import "time"

// Severity of an alert.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// RuleType identifies what kind of condition a rule checks.
type RuleType string

const (
	RuleTypePattern      RuleType = "pattern"
	RuleTypeLevel        RuleType = "level"
	RuleTypeHealthChange RuleType = "health_change"
	RuleTypeErrorRate    RuleType = "error_rate"
)

// Rule defines a single alert condition.
type Rule struct {
	Name      string
	Type      RuleType
	Severity  Severity
	Pattern   string        // for pattern rules: regex
	Level     string        // for level rules: min level
	Source    string        // optional: restrict to a specific source
	Threshold int           // for error_rate: errors per window
	Window    time.Duration // for error_rate: sliding window
	Cooldown  time.Duration // min time between repeated firings
}

// Event represents a fired alert.
type Event struct {
	Rule      string    `json:"rule"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count,omitempty"`
}
