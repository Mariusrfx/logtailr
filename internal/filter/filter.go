package filter

import (
	"fmt"
	"logtailr/pkg/logline"
	"regexp"
	"strings"
)

// RegexFilter holds a precompiled regex pattern for efficient repeated matching.
type RegexFilter struct {
	compiled *regexp.Regexp
}

// NewRegexFilter compiles and validates a regex pattern upfront.
// Returns an error if the pattern is invalid. An empty pattern matches everything.
func NewRegexFilter(pattern string) (*RegexFilter, error) {
	if pattern == "" {
		return &RegexFilter{}, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return &RegexFilter{compiled: re}, nil
}

// Match returns true if the text matches the compiled pattern.
// Always returns true if no pattern was set.
func (rf *RegexFilter) Match(text string) bool {
	if rf.compiled == nil {
		return true
	}
	return rf.compiled.MatchString(text)
}

// Filter checks if a log line passes both level and regex filters.
// Returns true if the line should be kept.
func Filter(line *logline.LogLine, minLevel, pattern string) (bool, error) {
	if !FilterByLevel(line, minLevel) {
		return false, nil
	}

	return FilterByRegex(line, pattern)
}

// FilterByLevel returns true if the log line's level is >= minLevel.
func FilterByLevel(line *logline.LogLine, minLevel string) bool {
	if minLevel == "" {
		return true
	}

	minLevelNum, ok := logline.LogLevels[strings.ToLower(minLevel)]
	if !ok {
		return true
	}

	lineLevel, ok := logline.LogLevels[strings.ToLower(line.Level)]
	if !ok {
		return true
	}

	return lineLevel >= minLevelNum
}

// FilterByRegex returns true if the log line's message matches the regex pattern.
// An empty pattern matches everything.
func FilterByRegex(line *logline.LogLine, pattern string) (bool, error) {
	if pattern == "" {
		return true, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}

	return re.MatchString(line.Message), nil
}
