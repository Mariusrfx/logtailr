package parser

import (
	"logtailr/pkg/logline"
	"testing"
)

func TestParseJSON_Valid(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name    string
		input   string
		wantLvl string
		wantMsg string
	}{
		{
			name:    "standard json",
			input:   `{"timestamp":"2024-01-15T10:30:00Z","level":"error","message":"Connection failed"}`,
			wantLvl: "error",
			wantMsg: "Connection failed",
		},
		{
			name:    "alternative keys",
			input:   `{"time":"2024-01-15T10:30:00Z","lvl":"warn","msg":"Warning message"}`,
			wantLvl: "warn",
			wantMsg: "Warning message",
		},
		{
			name:    "with extra fields",
			input:   `{"level":"info","message":"Request completed","user_id":123,"duration":1.5}`,
			wantLvl: "info",
			wantMsg: "Request completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll, err := p.ParseJSON(tt.input)
			if err != nil {
				t.Fatalf("ParseJSON() error = %v", err)
			}

			if ll.Level != tt.wantLvl {
				t.Errorf("Level = %q, want %q", ll.Level, tt.wantLvl)
			}
			if ll.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ll.Message, tt.wantMsg)
			}
			if ll.Source != "test.log" {
				t.Errorf("Source = %q, want %q", ll.Source, "test.log")
			}
		})
	}
}

func TestParseJSON_EmbeddedJSON(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name    string
		input   string
		wantLvl string
		wantMsg string
	}{
		{
			name:    "fluentd docker wrapper",
			input:   `2026-03-09T11:20:57.753854996Z 2026-03-09 11:20:57.753659809 +0000 all.udp: {"level":"error","message":"Connection refused","host":"67b3dedeb0b4"}`,
			wantLvl: "error",
			wantMsg: "Connection refused",
		},
		{
			name:    "simple prefix",
			input:   `INFO: {"level":"warn","message":"Slow query","duration":2.5}`,
			wantLvl: "warn",
			wantMsg: "Slow query",
		},
		{
			name:    "docker timestamp prefix",
			input:   `2026-03-09T11:20:57Z {"timestamp":"2026-03-09T11:20:57Z","level":"fatal","message":"OOM killed"}`,
			wantLvl: "fatal",
			wantMsg: "OOM killed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll, err := p.ParseJSON(tt.input)
			if err != nil {
				t.Fatalf("ParseJSON() error = %v", err)
			}

			if ll.Level != tt.wantLvl {
				t.Errorf("Level = %q, want %q", ll.Level, tt.wantLvl)
			}
			if ll.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ll.Message, tt.wantMsg)
			}
		})
	}
}

func TestParseJSON_Invalid(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"not json", "this is not json"},
		{"invalid json", `{"level": "error"`},
		{"array instead of object", `["error", "message"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseJSON(tt.input)
			if err == nil {
				t.Error("ParseJSON() expected error, got nil")
			}
		})
	}
}

func TestParseLogfmt_Valid(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name    string
		input   string
		wantLvl string
		wantMsg string
	}{
		{
			name:    "standard logfmt",
			input:   `time=2024-01-15T10:30:00Z level=error msg="Connection failed"`,
			wantLvl: "error",
			wantMsg: "Connection failed",
		},
		{
			name:    "unquoted message",
			input:   `level=info msg=simple`,
			wantLvl: "info",
			wantMsg: "simple",
		},
		{
			name:    "with extra fields",
			input:   `level=warn message="Slow query" duration=2.5 query="SELECT *"`,
			wantLvl: "warn",
			wantMsg: "Slow query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll, err := p.ParseLogfmt(tt.input)
			if err != nil {
				t.Fatalf("ParseLogfmt() error = %v", err)
			}

			if ll.Level != tt.wantLvl {
				t.Errorf("Level = %q, want %q", ll.Level, tt.wantLvl)
			}
			if ll.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ll.Message, tt.wantMsg)
			}
		})
	}
}

func TestParseText_WithTimestamp(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name    string
		input   string
		wantLvl string
		wantMsg string
	}{
		{
			name:    "bracketed timestamp",
			input:   `[2024-01-15 10:30:00] ERROR: Connection failed`,
			wantLvl: "error",
			wantMsg: "Connection failed",
		},
		{
			name:    "space separated",
			input:   `2024-01-15T10:30:00Z ERROR Database unavailable`,
			wantLvl: "error",
			wantMsg: "Database unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll, err := p.ParseText(tt.input)
			if err != nil {
				t.Fatalf("ParseText() error = %v", err)
			}

			if ll.Level != tt.wantLvl {
				t.Errorf("Level = %q, want %q", ll.Level, tt.wantLvl)
			}
			if ll.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ll.Message, tt.wantMsg)
			}
		})
	}
}

func TestParseText_WithoutTimestamp(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name    string
		input   string
		wantLvl string
		wantMsg string
	}{
		{
			name:    "level prefix with colon",
			input:   `ERROR: Something went wrong`,
			wantLvl: "error",
			wantMsg: "Something went wrong",
		},
		{
			name:    "plain message",
			input:   `Just a plain log message without level`,
			wantLvl: "info",
			wantMsg: "Just a plain log message without level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll, err := p.ParseText(tt.input)
			if err != nil {
				t.Fatalf("ParseText() error = %v", err)
			}

			if ll.Level != tt.wantLvl {
				t.Errorf("Level = %q, want %q", ll.Level, tt.wantLvl)
			}
			if ll.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ll.Message, tt.wantMsg)
			}
		})
	}
}

func TestParse_AutoDetect(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name    string
		input   string
		wantLvl string
		wantMsg string
	}{
		{
			name:    "detects json",
			input:   `{"level":"error","message":"JSON error"}`,
			wantLvl: "error",
			wantMsg: "JSON error",
		},
		{
			name:    "detects logfmt",
			input:   `level=warn msg="Logfmt warning" code=500`,
			wantLvl: "warn",
			wantMsg: "Logfmt warning",
		},
		{
			name:    "detects embedded json",
			input:   `2026-03-09T11:20:57Z all.udp: {"level":"error","message":"Embedded JSON"}`,
			wantLvl: "error",
			wantMsg: "Embedded JSON",
		},
		{
			name:    "falls back to text",
			input:   `[2024-01-15 10:30:00] INFO: Text log`,
			wantLvl: "info",
			wantMsg: "Text log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll, err := p.AutoDetect(tt.input)
			if err != nil {
				t.Fatalf("AutoDetect() error = %v", err)
			}

			if ll.Level != tt.wantLvl {
				t.Errorf("Level = %q, want %q", ll.Level, tt.wantLvl)
			}
			if ll.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ll.Message, tt.wantMsg)
			}
		})
	}
}

func TestParse_WithFormat(t *testing.T) {
	p := New("test.log")

	t.Run("explicit JSON format", func(t *testing.T) {
		ll, err := p.Parse(`{"level":"error","message":"test"}`, logline.ParserJSON)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if ll.Level != "error" {
			t.Errorf("Level = %q, want %q", ll.Level, "error")
		}
	})

	t.Run("auto format detection", func(t *testing.T) {
		ll, err := p.Parse(`level=info msg="auto test"`, "")
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if ll.Level != "info" {
			t.Errorf("Level = %q, want %q", ll.Level, "info")
		}
	})
}

func TestParseEmpty(t *testing.T) {
	p := New("test.log")

	tests := []struct {
		name   string
		parser func(string) (*logline.LogLine, error)
	}{
		{"ParseJSON", p.ParseJSON},
		{"ParseLogfmt", p.ParseLogfmt},
		{"ParseText", p.ParseText},
		{"AutoDetect", p.AutoDetect},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.parser("")
			if err != ErrEmptyLine {
				t.Errorf("Expected ErrEmptyLine, got %v", err)
			}

			_, err = tt.parser("   ")
			if err != ErrEmptyLine {
				t.Errorf("Expected ErrEmptyLine for whitespace, got %v", err)
			}
		})
	}
}

func TestNormalizeLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ERROR", "error"},
		{"Warning", "warn"},
		{"warning", "warn"},
		{"err", "error"},
		{"CRIT", "fatal"},
		{"critical", "fatal"},
		{"trace", "debug"},
		{"information", "info"},
		{"unknown", "info"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLevel(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"RFC3339", "2024-01-15T10:30:00Z", false},
		{"date time", "2024-01-15 10:30:00", false},
		{"unix float", float64(1705315800), false},
		{"unix int", int64(1705315800), false},
		{"invalid", "not a date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := parseTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && ts.IsZero() {
				t.Error("parseTimestamp() returned zero time")
			}
		})
	}
}

func TestJSONExtraFields(t *testing.T) {
	p := New("test.log")

	input := `{"level":"info","message":"test","user_id":123,"request_id":"abc-123"}`
	ll, err := p.ParseJSON(input)
	if err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}

	if ll.Fields["user_id"] != float64(123) {
		t.Errorf("Fields[user_id] = %v, want 123", ll.Fields["user_id"])
	}
	if ll.Fields["request_id"] != "abc-123" {
		t.Errorf("Fields[request_id] = %v, want abc-123", ll.Fields["request_id"])
	}
}

func TestLogfmtExtraFields(t *testing.T) {
	p := New("test.log")

	input := `level=info msg="test" user_id=123 request_id=abc-123`
	ll, err := p.ParseLogfmt(input)
	if err != nil {
		t.Fatalf("ParseLogfmt() error = %v", err)
	}

	if ll.Fields["user_id"] != "123" {
		t.Errorf("Fields[user_id] = %v, want 123", ll.Fields["user_id"])
	}
	if ll.Fields["request_id"] != "abc-123" {
		t.Errorf("Fields[request_id] = %v, want abc-123", ll.Fields["request_id"])
	}
}
