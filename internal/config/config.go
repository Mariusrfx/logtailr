package config

import (
	"fmt"
	"logtailr/pkg/logline"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the full application configuration.
type Config struct {
	Sources []logline.SourceConfig `mapstructure:"sources"`
	Global  GlobalConfig           `mapstructure:"global"`
}

// GlobalConfig holds global settings.
type GlobalConfig struct {
	Level      string `mapstructure:"level"`
	Regex      string `mapstructure:"regex"`
	Output     string `mapstructure:"output"`
	OutputPath string `mapstructure:"output_path"`
	ShowHealth bool   `mapstructure:"show_health"`
}

var validSourceTypes = map[string]bool{
	logline.SourceTypeFile:       true,
	logline.SourceTypeDocker:     true,
	logline.SourceTypeJournalctl: true,
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

	return nil
}
