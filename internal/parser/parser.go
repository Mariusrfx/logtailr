package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"logtailr/pkg/logline"
	"regexp"
	"strings"
	"time"
)

var (
	ErrEmptyLine     = errors.New("empty line")
	ErrInvalidFormat = errors.New("invalid format")
	ErrLineTooLarge  = errors.New("log line exceeds maximum size")
	ErrTooManyFields = errors.New("too many fields in log entry")
)

const (
	maxLineSize    = 256 * 1024 // 256KB max per log line
	maxFieldsCount = 100        // max JSON/logfmt fields per entry
)

// Key aliases for common log fields
var (
	timestampKeys = []string{"timestamp", "time", "ts", "@timestamp", "datetime"}
	levelKeys     = []string{"level", "lvl", "severity", "loglevel"}
	messageKeys   = []string{"message", "msg", "text", "log"}
)

// Level aliases mapping to standard levels
var levelAliases = map[string]string{
	"warning":     "warn",
	"err":         "error",
	"crit":        "fatal",
	"critical":    "fatal",
	"trace":       "debug",
	"information": "info",
}

// Common timestamp formats to try when parsing
var timestampFormats = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"02/Jan/2006:15:04:05 -0700",
	"Jan 02 15:04:05",
}

// Precompiled regex patterns
var (
	// [2024-01-15 10:30:00] ERROR: message
	patternBracketed = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}[^\]]*)\]\s*(\w+):?\s*(.*)$`)
	// 2024-01-15 10:30:00 ERROR message
	patternSpaced = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}[^\s]*)\s+(\w+)\s+(.*)$`)
	// ERROR: message (no timestamp)
	patternLevelOnly = regexp.MustCompile(`^(\w+):?\s+(.*)$`)
	// logfmt key=value pattern
	patternLogfmt = regexp.MustCompile(`(\w+)=(?:"([^"]*)"|(\S*))`)
	// logfmt detection pattern
	patternLogfmtDetect = regexp.MustCompile(`\w+=(?:"[^"]*"|\S+)`)
)

const minLogfmtPairs = 2

// Parser handles parsing of different log formats
type Parser struct {
	source string
}

// New creates a new Parser with the given source name
func New(source string) *Parser {
	return &Parser{source: source}
}

// Parse parses a line using the specified format
func (p *Parser) Parse(line, format string) (*logline.LogLine, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, ErrEmptyLine
	}

	switch format {
	case logline.ParserJSON:
		return p.ParseJSON(line)
	case logline.ParserLogfmt:
		return p.ParseLogfmt(line)
	case logline.ParserText:
		return p.ParseText(line)
	default:
		return p.AutoDetect(line)
	}
}

// ParseJSON parses a JSON formatted log line
func (p *Parser) ParseJSON(line string) (*logline.LogLine, error) {
	if err := validateLine(line); err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}

	if len(raw) > maxFieldsCount {
		return nil, fmt.Errorf("%w: got %d, max %d", ErrTooManyFields, len(raw), maxFieldsCount)
	}

	ll := p.newLogLine()
	ll.Timestamp = extractTimestamp(raw, timestampKeys)
	ll.Level = extractLevel(raw, levelKeys)
	ll.Message = extractMessage(raw, messageKeys)
	ll.Fields = raw

	return ll, nil
}

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

// ParseText parses a plain text log line using common patterns
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

// AutoDetect tries to automatically detect the format and parse
func (p *Parser) AutoDetect(line string) (*logline.LogLine, error) {
	if err := validateLine(line); err != nil {
		return nil, err
	}

	if strings.HasPrefix(line, "{") {
		if ll, err := p.ParseJSON(line); err == nil {
			return ll, nil
		}
	}

	if isLogfmt(line) {
		if ll, err := p.ParseLogfmt(line); err == nil {
			return ll, nil
		}
	}

	return p.ParseText(line)
}

// Helper methods

func (p *Parser) newLogLine() *logline.LogLine {
	return &logline.LogLine{
		Source:    p.source,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
	}
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

func validateLine(line string) error {
	if strings.TrimSpace(line) == "" {
		return ErrEmptyLine
	}
	if len(line) > maxLineSize {
		return fmt.Errorf("%w: %d bytes", ErrLineTooLarge, len(line))
	}
	return nil
}

// Extraction helpers for JSON

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

// Extraction helpers for logfmt pairs

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

// Parsing helpers

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
