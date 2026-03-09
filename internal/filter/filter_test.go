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

func TestByLevel_DebugShowsAll(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "fatal"}
	for _, lvl := range levels {
		line := newTestLine(lvl, "test")
		if !ByLevel(line, "debug") {
			t.Errorf("ByLevel(%q, debug) = false, want true", lvl)
		}
	}
}

func TestByLevel_ErrorShowsOnlyErrorAndFatal(t *testing.T) {
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
			got := ByLevel(line, "error")
			if got != tt.want {
				t.Errorf("ByLevel(%q, error) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestByLevel_WarnShowsWarnErrorFatal(t *testing.T) {
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
			got := ByLevel(line, "warn")
			if got != tt.want {
				t.Errorf("ByLevel(%q, warn) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestByLevel_EmptyMinLevel(t *testing.T) {
	line := newTestLine("debug", "test")
	if !ByLevel(line, "") {
		t.Error("ByLevel with empty minLevel should return true")
	}
}

func TestByLevel_InvalidMinLevel(t *testing.T) {
	line := newTestLine("info", "test")
	if !ByLevel(line, "invalid") {
		t.Error("ByLevel with invalid minLevel should return true")
	}
}

func TestByRegex_Match(t *testing.T) {
	line := newTestLine("error", "Connection failed to database")

	ok, err := ByRegex(line, "Connection.*database")
	if err != nil {
		t.Fatalf("ByRegex() error = %v", err)
	}
	if !ok {
		t.Error("ByRegex() = false, want true")
	}
}

func TestByRegex_NoMatch(t *testing.T) {
	line := newTestLine("info", "Server started successfully")

	ok, err := ByRegex(line, "error|failed")
	if err != nil {
		t.Fatalf("ByRegex() error = %v", err)
	}
	if ok {
		t.Error("ByRegex() = true, want false")
	}
}

func TestByRegex_EmptyPattern(t *testing.T) {
	line := newTestLine("info", "anything")

	ok, err := ByRegex(line, "")
	if err != nil {
		t.Fatalf("ByRegex() error = %v", err)
	}
	if !ok {
		t.Error("ByRegex with empty pattern should return true")
	}
}

func TestByRegex_InvalidRegex(t *testing.T) {
	line := newTestLine("info", "test")

	_, err := ByRegex(line, "[invalid")
	if err == nil {
		t.Error("ByRegex() expected error for invalid regex, got nil")
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
			got, err := Apply(line, tt.minLvl, tt.pattern)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_InvalidRegex(t *testing.T) {
	line := newTestLine("error", "test")
	_, err := Apply(line, "debug", "[invalid")
	if err == nil {
		t.Error("Apply() expected error for invalid regex, got nil")
	}
}
