package api

import (
	"regexp"
	"strings"
)

// safeLabel matches valid Prometheus label values (alphanumeric, dash, underscore, dot, slash, colon).
var safeLabel = regexp.MustCompile(`^[a-zA-Z0-9/_.:@-]+$`)

// sanitizeInput truncates and strips non-printable characters from user input.
func sanitizeInput(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	// Strip non-printable characters
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}

// SanitizeLabel returns a safe string for use as a Prometheus label value.
// Invalid values are replaced with "unknown".
func SanitizeLabel(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	if s == "" || !safeLabel.MatchString(s) {
		return "unknown"
	}
	return s
}
