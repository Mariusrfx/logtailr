package parser

import (
	"encoding/json"
	"fmt"
	"logtailr/pkg/logline"
	"strings"
)

// ParseJSON parses a JSON formatted log line.
// If the line contains embedded JSON (e.g. Fluentd/Docker prefix before a JSON object),
// it extracts and parses the JSON portion.
func (p *Parser) ParseJSON(line string) (*logline.LogLine, error) {
	if err := validateLine(line); err != nil {
		return nil, err
	}

	jsonStr := line

	// If line doesn't start with '{', look for embedded JSON
	if !strings.HasPrefix(strings.TrimSpace(line), "{") {
		idx := strings.Index(line, "{")
		if idx < 0 {
			return nil, ErrInvalidFormat
		}
		jsonStr = line[idx:]
	}

	// Use streaming decoder to avoid full in-memory copy
	var raw map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	if err := decoder.Decode(&raw); err != nil {
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
