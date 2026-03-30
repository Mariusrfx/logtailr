package cmd

import (
	"context"
	"fmt"
	"logtailr/internal/alert"
	"logtailr/internal/config"
	"logtailr/internal/health"
	"logtailr/internal/store"
	"os"
	"strings"
	"time"
)

func buildAlertEngine(cfg *config.AlertsConfig, monitor *health.Monitor) (*alert.Engine, error) {
	rules, err := convertAlertRules(cfg)
	if err != nil {
		return nil, err
	}

	notifiers := buildAlertNotifiers(cfg)

	engine, err := alert.NewEngine(rules, notifiers)
	if err != nil {
		return nil, err
	}

	monitor.SetOnChange(func(source string, oldStatus, newStatus health.Status) {
		engine.ProcessHealthChange(source, oldStatus, newStatus)
	})

	return engine, nil
}

func convertAlertRules(cfg *config.AlertsConfig) ([]alert.Rule, error) {
	defaultCooldown := 5 * time.Minute
	if cfg.DefaultCooldown != "" {
		d, err := time.ParseDuration(cfg.DefaultCooldown)
		if err != nil {
			return nil, fmt.Errorf("invalid default_cooldown: %w", err)
		}
		defaultCooldown = d
	}

	rules := make([]alert.Rule, 0, len(cfg.Rules))
	for _, rc := range cfg.Rules {
		cooldown := defaultCooldown
		if rc.Cooldown != "" {
			d, err := time.ParseDuration(rc.Cooldown)
			if err != nil {
				return nil, fmt.Errorf("rule %q: invalid cooldown: %w", rc.Name, err)
			}
			cooldown = d
		}

		var window time.Duration
		if rc.Window != "" {
			d, err := time.ParseDuration(rc.Window)
			if err != nil {
				return nil, fmt.Errorf("rule %q: invalid window: %w", rc.Name, err)
			}
			window = d
		}

		rules = append(rules, alert.Rule{
			Name:      rc.Name,
			Type:      alert.RuleType(rc.Type),
			Severity:  alert.Severity(strings.ToLower(rc.Severity)),
			Pattern:   rc.Pattern,
			Level:     rc.Level,
			Source:    rc.Source,
			Threshold: rc.Threshold,
			Window:    window,
			Cooldown:  cooldown,
		})
	}

	return rules, nil
}

func reloadAlertRulesFromDB(ctx context.Context, st *store.Store) ([]alert.Rule, error) {
	ruleRows, err := st.ListAlertRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("load alert rules: %w", err)
	}

	defaultCooldown := 5 * time.Minute
	if dc, err := config.LoadSettingString(ctx, st, "alerts.default_cooldown"); err == nil && dc != "" {
		if d, err := time.ParseDuration(dc); err == nil {
			defaultCooldown = d
		}
	}

	rules := make([]alert.Rule, 0, len(ruleRows))
	for _, r := range ruleRows {
		if !r.Enabled {
			continue
		}

		cooldown := defaultCooldown
		if r.Cooldown != "" {
			if d, err := time.ParseDuration(r.Cooldown); err == nil {
				cooldown = d
			}
		}

		var window time.Duration
		if r.Window != "" {
			if d, err := time.ParseDuration(r.Window); err == nil {
				window = d
			}
		}

		rules = append(rules, alert.Rule{
			Name:      r.Name,
			Type:      alert.RuleType(r.Type),
			Severity:  alert.Severity(strings.ToLower(r.Severity)),
			Pattern:   r.Pattern,
			Level:     r.Level,
			Source:    r.Source,
			Threshold: r.Threshold,
			Window:    window,
			Cooldown:  cooldown,
		})
	}

	return rules, nil
}

func buildAlertNotifiers(cfg *config.AlertsConfig) []alert.Notifier {
	var notifiers []alert.Notifier

	if cfg.Notify.Console {
		notifiers = append(notifiers, alert.NewConsoleNotifier())
	}

	if cfg.Notify.Webhook != nil && cfg.Notify.Webhook.URL != "" {
		notifiers = append(notifiers, alert.NewWebhookNotifier(cfg.Notify.Webhook.URL))
	}

	if cfg.Notify.Email != nil && cfg.Notify.Email.Host != "" {
		emailNotifier, err := alert.NewEmailNotifier(alert.EmailConfig{
			Host:     cfg.Notify.Email.Host,
			Port:     cfg.Notify.Email.Port,
			From:     cfg.Notify.Email.From,
			To:       cfg.Notify.Email.To,
			Username: cfg.Notify.Email.Username,
			Password: cfg.Notify.Email.Password,
			TLS:      cfg.Notify.Email.TLS,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: email notifier disabled: %v\n", err)
		} else {
			notifiers = append(notifiers, emailNotifier)
		}
	}

	if len(notifiers) == 0 {
		notifiers = append(notifiers, alert.NewConsoleNotifier())
	}

	return notifiers
}
