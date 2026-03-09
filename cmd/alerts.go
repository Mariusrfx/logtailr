package cmd

import (
	"fmt"
	"logtailr/internal/alert"
	"logtailr/internal/config"
	"logtailr/internal/health"
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

	// Wire health change callback
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

func buildAlertNotifiers(cfg *config.AlertsConfig) []alert.Notifier {
	var notifiers []alert.Notifier

	if cfg.Notify.Console {
		notifiers = append(notifiers, alert.NewConsoleNotifier())
	}

	if cfg.Notify.Webhook != nil && cfg.Notify.Webhook.URL != "" {
		notifiers = append(notifiers, alert.NewWebhookNotifier(cfg.Notify.Webhook.URL))
	}

	// Default to console if no notifier configured
	if len(notifiers) == 0 {
		notifiers = append(notifiers, alert.NewConsoleNotifier())
	}

	return notifiers
}
