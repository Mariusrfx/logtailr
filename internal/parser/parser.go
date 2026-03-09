package parser

import (
	"errors"
	"logtailr/pkg/logline"
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

func (p *Parser) newLogLine() *logline.LogLine {
	return &logline.LogLine{
		Source:    p.source,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
	}
}
