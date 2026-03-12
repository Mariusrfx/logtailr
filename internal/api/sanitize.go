package api

import (
	"regexp"
	"strings"
)

var safeLabel = regexp.MustCompile(`^[a-zA-Z0-9/_.:@-]+$`)

func sanitizeInput(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}

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
