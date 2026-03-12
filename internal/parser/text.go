package parser

import (
	"logtailr/pkg/logline"
	"regexp"
	"strings"
)

// Precompiled text patterns
var (
	// [2024-01-15 10:30:00] ERROR: message
	patternBracketed = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}[^]]*)]\s*(\w+):?\s*(.*)$`)
	// 2024-01-15 10:30:00 ERROR message
	patternSpaced = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}\S*)\s+(\w+)\s+(.*)$`)
	// ERROR: message (no timestamp)
	patternLevelOnly = regexp.MustCompile(`^(\w+):?\s+(.*)$`)
)

func (p *Parser) ParseText(line string) (*logline.LogLine, error) {
	if err := validateLine(line); err != nil {
		return nil, err
	}

	ll := p.newLogLine()

	if ok := p.parseWithTimestampPatterns(line, ll); ok {
		return ll, nil
	}

	if ok := p.parseLevelOnly(line, ll); ok {
		return ll, nil
	}

	ll.Message = line
	ll.Level = "info"
	return ll, nil
}

func (p *Parser) parseWithTimestampPatterns(line string, ll *logline.LogLine) bool {
	patterns := []*regexp.Regexp{patternBracketed, patternSpaced}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(line); matches != nil {
			if ts, err := parseTimestampString(matches[1]); err == nil {
				ll.Timestamp = ts
			}
			ll.Level = normalizeLevel(matches[2])
			ll.Message = strings.TrimSpace(matches[3])
			return true
		}
	}
	return false
}

func (p *Parser) parseLevelOnly(line string, ll *logline.LogLine) bool {
	matches := patternLevelOnly.FindStringSubmatch(line)
	if matches == nil {
		return false
	}

	rawLevel := strings.ToLower(strings.TrimSpace(matches[1]))
	if !isValidLogLevel(rawLevel) {
		return false
	}

	ll.Level = normalizeLevel(matches[1])
	ll.Message = strings.TrimSpace(matches[2])
	return true
}
