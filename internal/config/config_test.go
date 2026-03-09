package config

import (
	"logtailr/pkg/logline"
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfig_ValidYAML(t *testing.T) {
	path := writeConfigFile(t, `
sources:
  - name: "app-logs"
    type: "file"
    path: "/var/log/app.log"
    follow: true
    parser: "json"
  - name: "nginx"
    type: "file"
    path: "/var/log/nginx/access.log"
    parser: "text"
global:
  level: "info"
  output: "console"
  show_health: true
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(cfg.Sources) != 2 {
		t.Fatalf("got %d sources, want 2", len(cfg.Sources))
	}
	if cfg.Sources[0].Name != "app-logs" {
		t.Errorf("Sources[0].Name = %q, want %q", cfg.Sources[0].Name, "app-logs")
	}
	if cfg.Sources[0].Parser != "json" {
		t.Errorf("Sources[0].Parser = %q, want %q", cfg.Sources[0].Parser, "json")
	}
	if !cfg.Sources[0].Follow {
		t.Error("Sources[0].Follow = false, want true")
	}
	if cfg.Global.Level != "info" {
		t.Errorf("Global.Level = %q, want %q", cfg.Global.Level, "info")
	}
	if !cfg.Global.ShowHealth {
		t.Error("Global.ShowHealth = false, want true")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := writeConfigFile(t, `
sources:
  - name: "test
    type: [broken
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestValidateConfig_MissingName(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Type: "file",
			Path: "/var/log/app.log",
		}},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestValidateConfig_InvalidType(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "invalid",
		}},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestValidateConfig_DuplicateNames(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{
			{Name: "dup", Type: "file", Path: "/a.log"},
			{Name: "dup", Type: "file", Path: "/b.log"},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for duplicate names, got nil")
	}
}

func TestValidateConfig_FileMissingPath(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "file",
		}},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for file without path, got nil")
	}
}

func TestValidateConfig_DockerMissingContainer(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "docker",
		}},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for docker without container, got nil")
	}
}

func TestValidateConfig_InvalidParser(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name:   "test",
			Type:   "file",
			Path:   "/a.log",
			Parser: "xml",
		}},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid parser, got nil")
	}
}

func TestValidateConfig_InvalidLevel(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "file",
			Path: "/a.log",
		}},
		Global: GlobalConfig{Level: "verbose"},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid level, got nil")
	}
}

func TestValidateConfig_InvalidOutput(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "file",
			Path: "/a.log",
		}},
		Global: GlobalConfig{Output: "kafka"},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid output, got nil")
	}
}

func TestValidateConfig_FileOutputMissingPath(t *testing.T) {
	cfg := &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "file",
			Path: "/a.log",
		}},
		Global: GlobalConfig{Output: "file"},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for file output without path, got nil")
	}
}

func TestValidateConfig_NoSources(t *testing.T) {
	cfg := &Config{}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for empty sources, got nil")
	}
}

func validBaseConfig() *Config {
	return &Config{
		Sources: []logline.SourceConfig{{
			Name: "test",
			Type: "file",
			Path: "/a.log",
		}},
	}
}

func TestValidateConfig_AlertsValid(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled:         true,
		DefaultCooldown: "5m",
		Notify:          AlertNotifyConfig{Console: true},
		Rules: []AlertRuleConfig{
			{Name: "fatal", Type: "level", Severity: "critical", Level: "fatal"},
			{Name: "pattern", Type: "pattern", Severity: "warning", Pattern: "OOM"},
			{Name: "rate", Type: "error_rate", Severity: "warning", Threshold: 10, Window: "5m"},
			{Name: "health", Type: "health_change", Severity: "critical"},
		},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestValidateConfig_AlertsNoRules(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{Enabled: true, Notify: AlertNotifyConfig{Console: true}}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for alerts with no rules")
	}
}

func TestValidateConfig_AlertsDuplicateRuleName(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules: []AlertRuleConfig{
			{Name: "r1", Type: "level", Severity: "critical", Level: "fatal"},
			{Name: "r1", Type: "level", Severity: "warning", Level: "error"},
		},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for duplicate rule name")
	}
}

func TestValidateConfig_AlertsInvalidRuleType(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules:   []AlertRuleConfig{{Name: "r", Type: "unknown", Severity: "warning"}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid rule type")
	}
}

func TestValidateConfig_AlertsInvalidSeverity(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules:   []AlertRuleConfig{{Name: "r", Type: "level", Severity: "high", Level: "error"}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid severity")
	}
}

func TestValidateConfig_AlertsPatternMissing(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules:   []AlertRuleConfig{{Name: "r", Type: "pattern", Severity: "warning"}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for pattern rule without pattern")
	}
}

func TestValidateConfig_AlertsInvalidRegex(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules:   []AlertRuleConfig{{Name: "r", Type: "pattern", Severity: "warning", Pattern: "[invalid"}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestValidateConfig_AlertsErrorRateMissingThreshold(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules:   []AlertRuleConfig{{Name: "r", Type: "error_rate", Severity: "warning", Window: "5m"}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for error_rate without threshold")
	}
}

func TestValidateConfig_AlertsErrorRateMissingWindow(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled: true,
		Notify:  AlertNotifyConfig{Console: true},
		Rules:   []AlertRuleConfig{{Name: "r", Type: "error_rate", Severity: "warning", Threshold: 10}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for error_rate without window")
	}
}

func TestValidateConfig_AlertsInvalidCooldown(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{
		Enabled:         true,
		DefaultCooldown: "not-a-duration",
		Notify:          AlertNotifyConfig{Console: true},
		Rules:           []AlertRuleConfig{{Name: "r", Type: "health_change", Severity: "critical"}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid default_cooldown")
	}
}

func TestValidateConfig_AlertsDisabledSkipsValidation(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Alerts = &AlertsConfig{Enabled: false} // no rules, but disabled so ok
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("disabled alerts should not be validated, got: %v", err)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	path := writeConfigFile(t, `
sources:
  - name: "test"
    type: "file"
    path: "/var/log/test.log"
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Global.Level != "info" {
		t.Errorf("default level = %q, want %q", cfg.Global.Level, "info")
	}
	if cfg.Global.Output != "console" {
		t.Errorf("default output = %q, want %q", cfg.Global.Output, "console")
	}
}
