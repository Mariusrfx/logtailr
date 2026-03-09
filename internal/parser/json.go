package parser

import (
	"encoding/json"
	"fmt"
	"logtailr/pkg/logline"
	"strings"
)

// ParseJSON parses a JSON formatted log line
func (p *Parser) ParseJSON(line string) (*logline.LogLine, error) {
	if err := validateLine(line); err != nil {
		return nil, err
	}

	// Use streaming decoder to avoid full in-memory copy
	var raw map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(line))
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
