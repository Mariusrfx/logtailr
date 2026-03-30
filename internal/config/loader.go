package config

import (
	"context"
	"encoding/json"
	"fmt"

	"logtailr/internal/store"
	"logtailr/pkg/logline"
)

// LoadFromStore builds a Config from the database store.
// It loads sources, outputs, alert rules, and global settings.
func LoadFromStore(ctx context.Context, st *store.Store) (*Config, error) {
	cfg := &Config{}

	// Load sources
	sourceRows, err := st.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("load from store: sources: %w", err)
	}
	for _, r := range sourceRows {
		cfg.Sources = append(cfg.Sources, logline.SourceConfig{
			Name:          r.Name,
			Type:          r.Type,
			Path:          r.Path,
			Container:     r.Container,
			Unit:          r.Unit,
			Priority:      r.Priority,
			OutputFormat:  r.OutputFormat,
			Namespace:     r.Namespace,
			Pod:           r.Pod,
			LabelSelector: r.LabelSelector,
			Kubeconfig:    r.Kubeconfig,
			Follow:        r.Follow,
			Parser:        r.Parser,
		})
	}

	// Load outputs
	outputRows, err := st.ListOutputs(ctx)
	if err != nil {
		return nil, fmt.Errorf("load from store: outputs: %w", err)
	}
	for _, r := range outputRows {
		if !r.Enabled {
			continue
		}
		switch r.Type {
		case "opensearch":
			var osCfg OpenSearchOutputConfig
			if err := json.Unmarshal(r.Config, &osCfg); err != nil {
				return nil, fmt.Errorf("load from store: output %q config: %w", r.Name, err)
			}
			osCfg.Enabled = true
			cfg.Outputs.OpenSearch = &osCfg
		case "webhook":
			var whCfg WebhookOutputConfig
			if err := json.Unmarshal(r.Config, &whCfg); err != nil {
				return nil, fmt.Errorf("load from store: output %q config: %w", r.Name, err)
			}
			whCfg.Enabled = true
			cfg.Outputs.Webhook = &whCfg
		case "file":
			var fCfg FileOutputConfig
			if err := json.Unmarshal(r.Config, &fCfg); err != nil {
				return nil, fmt.Errorf("load from store: output %q config: %w", r.Name, err)
			}
			cfg.Outputs.File = &fCfg
		}
	}

	// Load alert rules
	ruleRows, err := st.ListAlertRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("load from store: alert rules: %w", err)
	}
	if len(ruleRows) > 0 {
		cfg.Alerts = &AlertsConfig{Enabled: true}
		for _, r := range ruleRows {
			if !r.Enabled {
				continue
			}
			cfg.Alerts.Rules = append(cfg.Alerts.Rules, AlertRuleConfig{
				Name:      r.Name,
				Type:      r.Type,
				Severity:  r.Severity,
				Pattern:   r.Pattern,
				Level:     r.Level,
				Source:    r.Source,
				Threshold: r.Threshold,
				Window:    r.Window,
				Cooldown:  r.Cooldown,
			})
		}
	}

	// Load global settings
	cfg.Global, err = loadGlobalSettings(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("load from store: settings: %w", err)
	}

	// Load alert notify settings
	if cfg.Alerts != nil {
		notifyCfg, err := loadAlertNotifySettings(ctx, st)
		if err != nil {
			return nil, fmt.Errorf("load from store: alert notify: %w", err)
		}
		cfg.Alerts.Notify = notifyCfg
		if dc, err := LoadSettingString(ctx, st, "alerts.default_cooldown"); err == nil && dc != "" {
			cfg.Alerts.DefaultCooldown = dc
		}
	}

	return cfg, nil
}

func loadGlobalSettings(ctx context.Context, st *store.Store) (GlobalConfig, error) {
	var g GlobalConfig

	if v, err := LoadSettingString(ctx, st, "global.level"); err == nil && v != "" {
		g.Level = v
	}
	if v, err := LoadSettingString(ctx, st, "global.regex"); err == nil && v != "" {
		g.Regex = v
	}
	if v, err := LoadSettingString(ctx, st, "global.output"); err == nil && v != "" {
		g.Output = v
	}
	if v, err := LoadSettingString(ctx, st, "global.output_path"); err == nil && v != "" {
		g.OutputPath = v
	}
	if v, err := loadSettingBool(ctx, st, "global.show_health"); err == nil {
		g.ShowHealth = v
	}
	if v, err := loadSettingBool(ctx, st, "global.aggregate"); err == nil {
		g.Aggregate = v
	}
	if v, err := LoadSettingString(ctx, st, "global.aggregate_window"); err == nil && v != "" {
		g.AggregateWindow = v
	}

	return g, nil
}

func loadAlertNotifySettings(ctx context.Context, st *store.Store) (AlertNotifyConfig, error) {
	var notify AlertNotifyConfig

	if v, err := loadSettingBool(ctx, st, "alerts.notify.console"); err == nil {
		notify.Console = v
	}
	if url, err := LoadSettingString(ctx, st, "alerts.notify.webhook.url"); err == nil && url != "" {
		notify.Webhook = &AlertWebhookConfig{URL: url}
	}

	return notify, nil
}

// LoadSettingString reads a JSON-encoded string setting from the store.
func LoadSettingString(ctx context.Context, st *store.Store, key string) (string, error) {
	raw, err := st.GetSetting(ctx, key)
	if err != nil || raw == nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func loadSettingBool(ctx context.Context, st *store.Store, key string) (bool, error) {
	raw, err := st.GetSetting(ctx, key)
	if err != nil || raw == nil {
		return false, err
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, err
	}
	return b, nil
}

// ImportToStore imports a YAML-loaded Config into the database store.
// Uses upsert semantics (ON CONFLICT ... DO UPDATE) for idempotent re-imports.
func ImportToStore(ctx context.Context, st *store.Store, cfg *Config) error {
	// Import sources
	for _, src := range cfg.Sources {
		row := &store.SourceRow{
			Name:          src.Name,
			Type:          src.Type,
			Path:          src.Path,
			Container:     src.Container,
			Unit:          src.Unit,
			Priority:      src.Priority,
			OutputFormat:  src.OutputFormat,
			Namespace:     src.Namespace,
			Pod:           src.Pod,
			LabelSelector: src.LabelSelector,
			Kubeconfig:    src.Kubeconfig,
			Follow:        src.Follow,
			Parser:        src.Parser,
		}
		if err := upsertSource(ctx, st, row); err != nil {
			return fmt.Errorf("import source %q: %w", src.Name, err)
		}
	}

	// Import outputs
	if err := importOutputs(ctx, st, &cfg.Outputs); err != nil {
		return err
	}

	// Import alert rules
	if cfg.Alerts != nil && cfg.Alerts.Enabled {
		for _, rule := range cfg.Alerts.Rules {
			row := &store.AlertRuleRow{
				Name:      rule.Name,
				Type:      rule.Type,
				Severity:  rule.Severity,
				Pattern:   rule.Pattern,
				Level:     rule.Level,
				Source:    rule.Source,
				Threshold: rule.Threshold,
				Window:    rule.Window,
				Cooldown:  rule.Cooldown,
				Enabled:   true,
			}
			if err := upsertAlertRule(ctx, st, row); err != nil {
				return fmt.Errorf("import alert rule %q: %w", rule.Name, err)
			}
		}

		// Import alert notify settings
		if cfg.Alerts.Notify.Console {
			if err := setSettingJSON(ctx, st, "alerts.notify.console", true); err != nil {
				return err
			}
		}
		if cfg.Alerts.Notify.Webhook != nil {
			if err := setSettingJSON(ctx, st, "alerts.notify.webhook.url", cfg.Alerts.Notify.Webhook.URL); err != nil {
				return err
			}
		}
		if cfg.Alerts.DefaultCooldown != "" {
			if err := setSettingJSON(ctx, st, "alerts.default_cooldown", cfg.Alerts.DefaultCooldown); err != nil {
				return err
			}
		}
	}

	// Import global settings
	return importGlobalSettings(ctx, st, &cfg.Global)
}

func importOutputs(ctx context.Context, st *store.Store, outputs *OutputsConfig) error {
	if outputs.OpenSearch != nil && outputs.OpenSearch.Enabled {
		cfgJSON, err := json.Marshal(outputs.OpenSearch)
		if err != nil {
			return fmt.Errorf("import output opensearch: %w", err)
		}
		row := &store.OutputRow{Name: "opensearch", Type: "opensearch", Config: cfgJSON, Enabled: true}
		if err := upsertOutput(ctx, st, row); err != nil {
			return fmt.Errorf("import output opensearch: %w", err)
		}
	}
	if outputs.Webhook != nil && outputs.Webhook.Enabled {
		cfgJSON, err := json.Marshal(outputs.Webhook)
		if err != nil {
			return fmt.Errorf("import output webhook: %w", err)
		}
		row := &store.OutputRow{Name: "webhook", Type: "webhook", Config: cfgJSON, Enabled: true}
		if err := upsertOutput(ctx, st, row); err != nil {
			return fmt.Errorf("import output webhook: %w", err)
		}
	}
	if outputs.File != nil {
		cfgJSON, err := json.Marshal(outputs.File)
		if err != nil {
			return fmt.Errorf("import output file: %w", err)
		}
		row := &store.OutputRow{Name: "file", Type: "file", Config: cfgJSON, Enabled: true}
		if err := upsertOutput(ctx, st, row); err != nil {
			return fmt.Errorf("import output file: %w", err)
		}
	}
	return nil
}

func importGlobalSettings(ctx context.Context, st *store.Store, g *GlobalConfig) error {
	settings := map[string]any{
		"global.level":            g.Level,
		"global.output":           g.Output,
		"global.output_path":      g.OutputPath,
		"global.show_health":      g.ShowHealth,
		"global.aggregate":        g.Aggregate,
		"global.aggregate_window": g.AggregateWindow,
	}
	if g.Regex != "" {
		settings["global.regex"] = g.Regex
	}

	for key, val := range settings {
		if err := setSettingJSON(ctx, st, key, val); err != nil {
			return fmt.Errorf("import setting %q: %w", key, err)
		}
	}
	return nil
}

func setSettingJSON(ctx context.Context, st *store.Store, key string, val any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return st.SetSetting(ctx, key, raw)
}

func upsertSource(ctx context.Context, st *store.Store, row *store.SourceRow) error {
	existing, err := st.GetSourceByName(ctx, row.Name)
	if err != nil {
		return st.CreateSource(ctx, row)
	}
	row.ID = existing.ID
	return st.UpdateSource(ctx, row)
}

func upsertOutput(ctx context.Context, st *store.Store, row *store.OutputRow) error {
	// Try create first, update on conflict
	err := st.CreateOutput(ctx, row)
	if err == nil {
		return nil
	}
	// Conflict — find existing by name and update
	outputs, listErr := st.ListOutputs(ctx)
	if listErr != nil {
		return err
	}
	for _, o := range outputs {
		if o.Name == row.Name {
			row.ID = o.ID
			return st.UpdateOutput(ctx, row)
		}
	}
	return err
}

func upsertAlertRule(ctx context.Context, st *store.Store, row *store.AlertRuleRow) error {
	// Try create first, update on conflict
	err := st.CreateAlertRule(ctx, row)
	if err == nil {
		return nil
	}
	// Conflict — find existing by name and update
	rules, listErr := st.ListAlertRules(ctx)
	if listErr != nil {
		return err
	}
	for _, r := range rules {
		if r.Name == row.Name {
			row.ID = r.ID
			return st.UpdateAlertRule(ctx, row)
		}
	}
	return err
}
