package config

import (
	"fmt"
	"logtailr/pkg/logline"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the full application configuration.
type Config struct {
	Sources []logline.SourceConfig `mapstructure:"sources"`
	Global  GlobalConfig           `mapstructure:"global"`
	Outputs OutputsConfig          `mapstructure:"outputs"`
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
