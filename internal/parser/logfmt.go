package parser

import (
	"fmt"
	"logtailr/pkg/logline"
	"regexp"
	"strings"
	"time"
)

// Precompiled logfmt patterns
var (
	patternLogfmt       = regexp.MustCompile(`(\w+)=(?:"([^"]*)"|(\S*))`)
	patternLogfmtDetect = regexp.MustCompile(`\w+=(?:"[^"]*"|\S+)`)
)

const minLogfmtPairs = 2

// ParseLogfmt parses a logfmt formatted log line (key=value pairs)
func (p *Parser) ParseLogfmt(line string) (*logline.LogLine, error) {
	if err := validateLine(line); err != nil {
		return nil, err
	}

	pairs := parseLogfmtPairs(line)

	if len(pairs) > maxFieldsCount {
		return nil, fmt.Errorf("%w: got %d, max %d", ErrTooManyFields, len(pairs), maxFieldsCount)
	}

	ll := p.newLogLine()

	ll.Timestamp = extractTimestampFromPairs(pairs, timestampKeys)
	ll.Level = extractLevelFromPairs(pairs, levelKeys)
	ll.Message = extractMessageFromPairs(pairs, messageKeys)

	for k, v := range pairs {
		ll.Fields[k] = v
	}

	return ll, nil
}

func parseLogfmtPairs(line string) map[string]string {
	pairs := make(map[string]string)
	matches := patternLogfmt.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		key := match[1]
		value := match[2]
		if value == "" {
			value = match[3]
		}
		pairs[key] = value
	}

	return pairs
}

func isLogfmt(line string) bool {
	if !strings.Contains(line, "=") {
		return false
	}
	matches := patternLogfmtDetect.FindAllString(line, -1)
	return len(matches) >= minLogfmtPairs
}

func extractTimestampFromPairs(pairs map[string]string, keys []string) time.Time {
	for _, key := range keys {
		if val, ok := pairs[key]; ok {
			if ts, err := parseTimestamp(val); err == nil {
				delete(pairs, key)
				return ts
			}
		}
	}
	return time.Now()
}

func extractLevelFromPairs(pairs map[string]string, keys []string) string {
	for _, key := range keys {
		if val, ok := pairs[key]; ok {
			delete(pairs, key)
			return normalizeLevel(val)
		}
	}
	return "info"
}

func extractMessageFromPairs(pairs map[string]string, keys []string) string {
	for _, key := range keys {
		if val, ok := pairs[key]; ok {
			delete(pairs, key)
			return val
		}
	}
	return ""
}
