package logline

import (
	"time"
)

// LogLine represents a parsed log line
type LogLine struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// SourceConfig defines the configuration for a log source
type SourceConfig struct {
	Name      string `mapstructure:"name"`
	Type      string `mapstructure:"type"` // "file", "docker", "journalctl"
	Path      string `mapstructure:"path"`
	Container string `mapstructure:"container"`
	Follow    bool   `mapstructure:"follow"`
	Parser    string `mapstructure:"parser"` // "json", "logfmt", "text"
}

// Parser type constants
const (
	ParserJSON   = "json"
	ParserLogfmt = "logfmt"
	ParserText   = "text"
)

// Source type constants
const (
	SourceTypeFile       = "file"
	SourceTypeDocker     = "docker"
	SourceTypeJournalctl = "journalctl"
)

// LogLevels maps log levels to their numeric severity
var LogLevels = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
	"fatal": 4,
}
