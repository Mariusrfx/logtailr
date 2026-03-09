package filter

import (
	"logtailr/pkg/logline"
	"testing"
	"time"
)

func newTestLine(level, message string) *logline.LogLine {
	return &logline.LogLine{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    "test.log",
	}
}

func TestFilterByLevel_DebugShowsAll(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "fatal"}
	for _, lvl := range levels {
		line := newTestLine(lvl, "test")
		if !FilterByLevel(line, "debug") {
			t.Errorf("FilterByLevel(%q, debug) = false, want true", lvl)
		}
	}
}

func TestFilterByLevel_ErrorShowsOnlyErrorAndFatal(t *testing.T) {
	tests := []struct {
		level string
		want  bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", true},
		{"fatal", true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			line := newTestLine(tt.level, "test")
			got := FilterByLevel(line, "error")
			if got != tt.want {
				t.Errorf("FilterByLevel(%q, error) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestFilterByLevel_WarnShowsWarnErrorFatal(t *testing.T) {
	tests := []struct {
		level string
		want  bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", true},
		{"error", true},
		{"fatal", true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			line := newTestLine(tt.level, "test")
			got := FilterByLevel(line, "warn")
			if got != tt.want {
				t.Errorf("FilterByLevel(%q, warn) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestFilterByLevel_EmptyMinLevel(t *testing.T) {
	line := newTestLine("debug", "test")
	if !FilterByLevel(line, "") {
		t.Error("FilterByLevel with empty minLevel should return true")
	}
}

func TestFilterByLevel_InvalidMinLevel(t *testing.T) {
	line := newTestLine("info", "test")
	if !FilterByLevel(line, "invalid") {
		t.Error("FilterByLevel with invalid minLevel should return true")
	}
}

func TestFilterByRegex_Match(t *testing.T) {
	line := newTestLine("error", "Connection failed to database")

	ok, err := FilterByRegex(line, "Connection.*database")
	if err != nil {
		t.Fatalf("FilterByRegex() error = %v", err)
	}
	if !ok {
		t.Error("FilterByRegex() = false, want true")
	}
}

func TestFilterByRegex_NoMatch(t *testing.T) {
	line := newTestLine("info", "Server started successfully")

	ok, err := FilterByRegex(line, "error|failed")
	if err != nil {
		t.Fatalf("FilterByRegex() error = %v", err)
	}
	if ok {
		t.Error("FilterByRegex() = true, want false")
	}
}

func TestFilterByRegex_EmptyPattern(t *testing.T) {
	line := newTestLine("info", "anything")

	ok, err := FilterByRegex(line, "")
	if err != nil {
		t.Fatalf("FilterByRegex() error = %v", err)
	}
	if !ok {
		t.Error("FilterByRegex with empty pattern should return true")
	}
}

func TestFilterByRegex_InvalidRegex(t *testing.T) {
	line := newTestLine("info", "test")

	_, err := FilterByRegex(line, "[invalid")
	if err == nil {
		t.Error("FilterByRegex() expected error for invalid regex, got nil")
	}
}

func TestFilter_Combined(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		message string
		minLvl  string
		pattern string
		want    bool
	}{
		{
			name:    "passes both",
			level:   "error",
			message: "Connection failed",
			minLvl:  "warn",
			pattern: "failed",
			want:    true,
		},
		{
			name:    "fails level",
			level:   "debug",
			message: "Connection failed",
			minLvl:  "warn",
			pattern: "failed",
			want:    false,
		},
		{
			name:    "fails regex",
			level:   "error",
			message: "Server started",
			minLvl:  "warn",
			pattern: "failed",
			want:    false,
		},
		{
			name:    "fails both",
			level:   "debug",
			message: "Server started",
			minLvl:  "warn",
			pattern: "failed",
			want:    false,
		},
		{
			name:    "no filters",
			level:   "debug",
			message: "anything",
			minLvl:  "",
			pattern: "",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := newTestLine(tt.level, tt.message)
			got, err := Filter(line, tt.minLvl, tt.pattern)
			if err != nil {
				t.Fatalf("Filter() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_InvalidRegex(t *testing.T) {
	line := newTestLine("error", "test")
	_, err := Filter(line, "debug", "[invalid")
	if err == nil {
		t.Error("Filter() expected error for invalid regex, got nil")
	}
}
