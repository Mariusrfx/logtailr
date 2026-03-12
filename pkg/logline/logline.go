package logline

import (
	"time"
)

type LogLine struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// SourceConfig defines the configuration for a log source
type SourceConfig struct {
	Name          string `mapstructure:"name"`
	Type          string `mapstructure:"type"`           // "file", "docker", "journalctl", "stdin", "kubernetes"
	Path          string `mapstructure:"path"`           // file path (type=file)
	Container     string `mapstructure:"container"`      // container name/id (type=docker) or container name (type=kubernetes)
	Unit          string `mapstructure:"unit"`           // systemd unit (type=journalctl)
	Priority      string `mapstructure:"priority"`       // journalctl priority filter (emerg..debug)
	OutputFormat  string `mapstructure:"output_format"`  // journalctl output format: "json" or "" (short-iso)
	Namespace     string `mapstructure:"namespace"`      // kubernetes namespace (type=kubernetes)
	Pod           string `mapstructure:"pod"`            // kubernetes pod name (type=kubernetes)
	LabelSelector string `mapstructure:"label_selector"` // kubernetes label selector (type=kubernetes), e.g. "app=myapp"
	Kubeconfig    string `mapstructure:"kubeconfig"`     // path to kubeconfig file (type=kubernetes)
	Follow        bool   `mapstructure:"follow"`
	Parser        string `mapstructure:"parser"` // "json", "logfmt", "text", "" (auto)
}

// JournalctlPriorities maps journalctl priority names to syslog numeric values.
var JournalctlPriorities = map[string]int{
	"emerg":   0,
	"alert":   1,
	"crit":    2,
	"err":     3,
	"warning": 4,
	"notice":  5,
	"info":    6,
	"debug":   7,
}

const (
	ParserJSON   = "json"
	ParserLogfmt = "logfmt"
	ParserText   = "text"
)

const (
	SourceTypeFile       = "file"
	SourceTypeDocker     = "docker"
	SourceTypeJournalctl = "journalctl"
	SourceTypeStdin      = "stdin"
	SourceTypeKubernetes = "kubernetes"
)

// LogLevels maps log levels to their numeric severity
var LogLevels = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
	"fatal": 4,
}
