package parser

import (
	"encoding/json"
	"fmt"
	"logtailr/pkg/logline"
	"strings"
	"time"
)

func validateLine(line string) error {
	if strings.TrimSpace(line) == "" {
		return ErrEmptyLine
	}
	if len(line) > maxLineSize {
		return fmt.Errorf("%w: %d bytes", ErrLineTooLarge, len(line))
	}
	return nil
}

func extractTimestamp(raw map[string]interface{}, keys []string) time.Time {
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			if ts, err := parseTimestamp(val); err == nil {
				delete(raw, key)
				return ts
			}
		}
	}
	return time.Now()
}

func extractLevel(raw map[string]interface{}, keys []string) string {
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			delete(raw, key)
			return normalizeLevel(toString(val))
		}
	}
	return "info"
}

func extractMessage(raw map[string]interface{}, keys []string) string {
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			delete(raw, key)
			return toString(val)
		}
	}
	return ""
}

func parseTimestamp(val interface{}) (time.Time, error) {
	switch v := val.(type) {
	case string:
		return parseTimestampString(v)
	case float64:
		return time.Unix(int64(v), 0), nil
	case int64:
		return time.Unix(v, 0), nil
	default:
		return time.Time{}, ErrInvalidFormat
	}
}

func parseTimestampString(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, format := range timestampFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, ErrInvalidFormat
}

func normalizeLevel(level string) string {
	level = strings.ToLower(strings.TrimSpace(level))

	if alias, ok := levelAliases[level]; ok {
		return alias
	}

	if _, ok := logline.LogLevels[level]; ok {
		return level
	}

	return "info"
}

func isValidLogLevel(level string) bool {
	level = strings.ToLower(strings.TrimSpace(level))

	if _, ok := logline.LogLevels[level]; ok {
		return true
	}

	_, isAlias := levelAliases[level]
	return isAlias
}

func toString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	}
}
