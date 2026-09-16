package api

import "testing"

func TestValidateAlertRuleRequest_Valid(t *testing.T) {
	req := &alertRuleRequest{
		Name:      "rule",
		Type:      "error_rate",
		Severity:  "critical",
		Threshold: 5,
		Window:    "30s",
		Cooldown:  "5m",
		Level:     "error",
	}
	if err := validateAlertRuleRequest(req); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateAlertRuleRequest_InvalidWindow(t *testing.T) {
	req := &alertRuleRequest{Name: "r", Type: "error_rate", Severity: "warning", Window: "abc"}
	if err := validateAlertRuleRequest(req); err == nil {
		t.Fatal("expected error for invalid window")
	}
}

func TestValidateAlertRuleRequest_InvalidCooldown(t *testing.T) {
	req := &alertRuleRequest{Name: "r", Type: "pattern", Severity: "warning", Cooldown: "10x"}
	if err := validateAlertRuleRequest(req); err == nil {
		t.Fatal("expected error for invalid cooldown")
	}
}

func TestValidateAlertRuleRequest_NegativeThreshold(t *testing.T) {
	req := &alertRuleRequest{Name: "r", Type: "error_rate", Severity: "warning", Threshold: -1}
	if err := validateAlertRuleRequest(req); err == nil {
		t.Fatal("expected error for negative threshold")
	}
}

func TestValidateAlertRuleRequest_InvalidLevel(t *testing.T) {
	req := &alertRuleRequest{Name: "r", Type: "level", Severity: "warning", Level: "noisy"}
	if err := validateAlertRuleRequest(req); err == nil {
		t.Fatal("expected error for unknown level")
	}
}
