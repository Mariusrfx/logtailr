package filter

import (
	"fmt"
	"logtailr/pkg/logline"
	"regexp"
	"strings"
)

type RegexFilter struct {
	compiled *regexp.Regexp
}

const maxRegexPatternLen = 1024

func NewRegexFilter(pattern string) (*RegexFilter, error) {
	if pattern == "" {
		return &RegexFilter{}, nil
	}

	if len(pattern) > maxRegexPatternLen {
		return nil, fmt.Errorf("regex pattern too long (%d chars, max %d)", len(pattern), maxRegexPatternLen)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return &RegexFilter{compiled: re}, nil
}

func (rf *RegexFilter) Match(text string) bool {
	if rf.compiled == nil {
		return true
	}
	return rf.compiled.MatchString(text)
}

func Apply(line *logline.LogLine, minLevel, pattern string) (bool, error) {
	if !ByLevel(line, minLevel) {
		return false, nil
	}

	return ByRegex(line, pattern)
}

func ByLevel(line *logline.LogLine, minLevel string) bool {
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

func ByRegex(line *logline.LogLine, pattern string) (bool, error) {
	if pattern == "" {
		return true, nil
	}

	rf, err := NewRegexFilter(pattern)
	if err != nil {
		return false, err
	}

	return rf.Match(line.Message), nil
}
