package config

import (
	"fmt"
	"logtailr/pkg/logline"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var validAlertRuleTypes = map[string]bool{
	"pattern":       true,
	"level":         true,
	"error_rate":    true,
	"health_change": true,
}

var validAlertSeverities = map[string]bool{
	"warning":  true,
	"critical": true,
}

// Config represents the full application configuration.
type Config struct {
	Sources []logline.SourceConfig `mapstructure:"sources"`
	Global  GlobalConfig           `mapstructure:"global"`
	Outputs OutputsConfig          `mapstructure:"outputs"`
	Alerts  *AlertsConfig          `mapstructure:"alerts"`
}

// AlertsConfig holds the alerts configuration.
type AlertsConfig struct {
	Enabled         bool              `mapstructure:"enabled"`
	DefaultCooldown string            `mapstructure:"default_cooldown"`
	Notify          AlertNotifyConfig `mapstructure:"notify"`
	Rules           []AlertRuleConfig `mapstructure:"rules"`
}

// AlertNotifyConfig holds alert notification destinations.
type AlertNotifyConfig struct {
	Console bool                `mapstructure:"console"`
	Webhook *AlertWebhookConfig `mapstructure:"webhook"`
}

// AlertWebhookConfig holds webhook settings for alerts.
type AlertWebhookConfig struct {
	URL string `mapstructure:"url"`
}

// AlertRuleConfig holds the configuration for a single alert rule.
type AlertRuleConfig struct {
	Name      string `mapstructure:"name"`
	Type      string `mapstructure:"type"`
	Severity  string `mapstructure:"severity"`
	Pattern   string `mapstructure:"pattern"`
	Level     string `mapstructure:"level"`
	Source    string `mapstructure:"source"`
	Threshold int    `mapstructure:"threshold"`
	Window    string `mapstructure:"window"`
	Cooldown  string `mapstructure:"cooldown"`
}

// GlobalConfig holds global settings.
type GlobalConfig struct {
	Level      string `mapstructure:"level"`
	Regex      string `mapstructure:"regex"`
	Output     string `mapstructure:"output"`
	OutputPath string `mapstructure:"output_path"`
	ShowHealth bool   `mapstructure:"show_health"`
}

// OutputsConfig holds configuration for output destinations.
type OutputsConfig struct {
	OpenSearch *OpenSearchOutputConfig `mapstructure:"opensearch"`
	Webhook    *WebhookOutputConfig    `mapstructure:"webhook"`
	File       *FileOutputConfig       `mapstructure:"file"`
}

// FileOutputConfig holds file output settings with rotation.
type FileOutputConfig struct {
	Path     string `mapstructure:"path"`
	MaxSize  string `mapstructure:"max_size"`
	MaxAge   string `mapstructure:"max_age"`
	Compress bool   `mapstructure:"compress"`
}

// OpenSearchOutputConfig holds OpenSearch connection settings.
type OpenSearchOutputConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	Hosts         []string `mapstructure:"hosts"`
	Index         string   `mapstructure:"index"`
	Username      string   `mapstructure:"username"`
	Password      string   `mapstructure:"password"`
	BulkSize      int      `mapstructure:"bulk_size"`
	FlushInterval string   `mapstructure:"flush_interval"`
	TLSSkipVerify bool     `mapstructure:"tls_skip_verify"`
	MaxRetries    int      `mapstructure:"max_retries"`
}

// WebhookOutputConfig holds webhook settings.
type WebhookOutputConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	URL          string `mapstructure:"url"`
	MinLevel     string `mapstructure:"min_level"`
	BatchSize    int    `mapstructure:"batch_size"`
	BatchTimeout string `mapstructure:"batch_timeout"`
}

var validSourceTypes = map[string]bool{
	logline.SourceTypeFile:       true,
	logline.SourceTypeDocker:     true,
	logline.SourceTypeJournalctl: true,
	logline.SourceTypeStdin:      true,
}

var validParsers = map[string]bool{
	logline.ParserJSON:   true,
	logline.ParserLogfmt: true,
	logline.ParserText:   true,
	"":                   true, // auto-detect
}

var validOutputs = map[string]bool{
	"console": true,
	"json":    true,
	"file":    true,
	"":        true, // defaults to console
}

// LoadConfig reads and parses a YAML config file.
func LoadConfig(path string) (*Config, error) {
	// Validate path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve config path: %w", err)
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access config file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("config path is not a regular file")
	}

	v := viper.New()
	v.SetConfigFile(absPath)
	v.SetConfigType("yaml")

	// Defaults
	v.SetDefault("global.level", "info")
	v.SetDefault("global.output", "console")
	v.SetDefault("global.show_health", false)

	// Environment variable support
	v.AutomaticEnv()
	v.SetEnvPrefix("LOGTAILR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := ValidateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ValidateConfig checks that the configuration is valid.
func ValidateConfig(cfg *Config) error {
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("config: at least one source is required")
	}

	seen := make(map[string]bool)
	for i, src := range cfg.Sources {
		if src.Name == "" {
			return fmt.Errorf("config: source[%d] is missing a name", i)
		}
		if seen[src.Name] {
			return fmt.Errorf("config: duplicate source name %q", src.Name)
		}
		seen[src.Name] = true

		if !validSourceTypes[src.Type] {
			return fmt.Errorf("config: source %q has invalid type %q (must be file, docker, or journalctl)", src.Name, src.Type)
		}

		if src.Type == logline.SourceTypeFile && src.Path == "" {
			return fmt.Errorf("config: source %q of type file requires a path", src.Name)
		}
		if src.Type == logline.SourceTypeDocker && src.Container == "" {
			return fmt.Errorf("config: source %q of type docker requires a container", src.Name)
		}
		if src.Type == logline.SourceTypeJournalctl && src.Unit == "" {
			return fmt.Errorf("config: source %q of type journalctl requires a unit", src.Name)
		}

		if !validParsers[src.Parser] {
			return fmt.Errorf("config: source %q has invalid parser %q (must be json, logfmt, or text)", src.Name, src.Parser)
		}
	}

	if _, ok := logline.LogLevels[strings.ToLower(cfg.Global.Level)]; cfg.Global.Level != "" && !ok {
		return fmt.Errorf("config: invalid global level %q", cfg.Global.Level)
	}

	if !validOutputs[cfg.Global.Output] {
		return fmt.Errorf("config: invalid output %q (must be console, json, or file)", cfg.Global.Output)
	}

	if cfg.Global.Output == "file" && cfg.Global.OutputPath == "" {
		return fmt.Errorf("config: output type file requires output_path")
	}

	if err := validateOutputsConfig(&cfg.Outputs); err != nil {
		return err
	}

	if cfg.Alerts != nil && cfg.Alerts.Enabled {
		if err := validateAlertsConfig(cfg.Alerts); err != nil {
			return err
		}
	}

	return nil
}

func validateOutputsConfig(outputs *OutputsConfig) error {
	if outputs.OpenSearch != nil && outputs.OpenSearch.Enabled {
		osCfg := outputs.OpenSearch
		if len(osCfg.Hosts) == 0 {
			return fmt.Errorf("config: opensearch requires at least one host")
		}
		if osCfg.Index == "" {
			return fmt.Errorf("config: opensearch requires an index")
		}
		for _, h := range osCfg.Hosts {
			if err := validateExternalURL(h); err != nil {
				return fmt.Errorf("config: opensearch host %q: %w", h, err)
			}
		}
	}

	if outputs.File != nil {
		fc := outputs.File
		if fc.Path == "" {
			return fmt.Errorf("config: file output requires a path")
		}
		if fc.MaxSize != "" {
			if _, err := ParseByteSize(fc.MaxSize); err != nil {
				return fmt.Errorf("config: file output has invalid max_size: %w", err)
			}
		}
		if fc.MaxAge != "" {
			if _, err := time.ParseDuration(fc.MaxAge); err != nil {
				return fmt.Errorf("config: file output has invalid max_age: %w", err)
			}
		}
	}

	if outputs.Webhook != nil && outputs.Webhook.Enabled {
		wh := outputs.Webhook
		if wh.URL == "" {
			return fmt.Errorf("config: webhook requires a url")
		}
		if err := validateExternalURL(wh.URL); err != nil {
			return fmt.Errorf("config: webhook url: %w", err)
		}
		if wh.MinLevel != "" {
			if _, ok := logline.LogLevels[strings.ToLower(wh.MinLevel)]; !ok {
				return fmt.Errorf("config: webhook has invalid min_level %q", wh.MinLevel)
			}
		}
	}

	return nil
}

func validateAlertsConfig(alerts *AlertsConfig) error {
	if alerts.DefaultCooldown != "" {
		if _, err := time.ParseDuration(alerts.DefaultCooldown); err != nil {
			return fmt.Errorf("config: alerts has invalid default_cooldown: %w", err)
		}
	}

	if alerts.Notify.Webhook != nil {
		if alerts.Notify.Webhook.URL == "" {
			return fmt.Errorf("config: alerts webhook requires a url")
		}
		if err := validateExternalURL(alerts.Notify.Webhook.URL); err != nil {
			return fmt.Errorf("config: alerts webhook url: %w", err)
		}
	}

	if len(alerts.Rules) == 0 {
		return fmt.Errorf("config: alerts enabled but no rules defined")
	}

	seenRules := make(map[string]bool)
	for i, rule := range alerts.Rules {
		if rule.Name == "" {
			return fmt.Errorf("config: alert rule[%d] is missing a name", i)
		}
		if seenRules[rule.Name] {
			return fmt.Errorf("config: duplicate alert rule name %q", rule.Name)
		}
		seenRules[rule.Name] = true

		if !validAlertRuleTypes[rule.Type] {
			return fmt.Errorf("config: alert rule %q has invalid type %q", rule.Name, rule.Type)
		}
		if !validAlertSeverities[rule.Severity] {
			return fmt.Errorf("config: alert rule %q has invalid severity %q (must be warning or critical)", rule.Name, rule.Severity)
		}

		if rule.Cooldown != "" {
			if _, err := time.ParseDuration(rule.Cooldown); err != nil {
				return fmt.Errorf("config: alert rule %q has invalid cooldown: %w", rule.Name, err)
			}
		}

		switch rule.Type {
		case "pattern":
			if rule.Pattern == "" {
				return fmt.Errorf("config: alert rule %q of type pattern requires a pattern", rule.Name)
			}
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return fmt.Errorf("config: alert rule %q has invalid pattern: %w", rule.Name, err)
			}
		case "level":
			if rule.Level == "" {
				return fmt.Errorf("config: alert rule %q of type level requires a level", rule.Name)
			}
			if _, ok := logline.LogLevels[strings.ToLower(rule.Level)]; !ok {
				return fmt.Errorf("config: alert rule %q has invalid level %q", rule.Name, rule.Level)
			}
		case "error_rate":
			if rule.Threshold <= 0 {
				return fmt.Errorf("config: alert rule %q of type error_rate requires a positive threshold", rule.Name)
			}
			if rule.Window == "" {
				return fmt.Errorf("config: alert rule %q of type error_rate requires a window", rule.Name)
			}
			if _, err := time.ParseDuration(rule.Window); err != nil {
				return fmt.Errorf("config: alert rule %q has invalid window: %w", rule.Name, err)
			}
		case "health_change":
			// No extra fields required
		}
	}

	return nil
}

// ParseByteSize parses a human-readable byte size string (e.g. "10MB", "500KB", "1GB").
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	s = strings.ToUpper(s)

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, m := range multipliers {
		if numStr, found := strings.CutSuffix(s, m.suffix); found {
			numStr = strings.TrimSpace(numStr)
			var n int64
			if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil || fmt.Sprintf("%d", n) != numStr {
				return 0, fmt.Errorf("invalid size number %q", numStr)
			}
			if n <= 0 {
				return 0, fmt.Errorf("size must be positive")
			}
			return n * m.mult, nil
		}
	}

	// Try plain number (bytes) — must be entirely numeric
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || fmt.Sprintf("%d", n) != s {
		return 0, fmt.Errorf("invalid size %q: use format like 10MB, 500KB, 1GB", s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("size must be positive")
	}
	return n, nil
}

// validateExternalURL checks that a URL is valid and not targeting internal/private networks (SSRF prevention).
func validateExternalURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("must start with http:// or https://")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Check for well-known dangerous hostnames
	if host == "localhost" || host == "metadata.google.internal" {
		return fmt.Errorf("internal hostname %q not allowed", host)
	}

	// Check if it's an IP address pointing to internal networks
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("internal/private IP address %q not allowed", host)
		}
	}

	return nil
}
