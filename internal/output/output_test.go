package output

import (
	"bytes"
	"encoding/json"
	"logtailr/pkg/logline"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedTime = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

func newTestLine(level, message string) *logline.LogLine {
	return &logline.LogLine{
		Timestamp: fixedTime,
		Level:     level,
		Message:   message,
		Source:    "app.log",
		Fields:    make(map[string]interface{}),
	}
}

// --- ConsoleWriter tests ---

func TestConsoleWriter_FormatOutput(t *testing.T) {
	var buf bytes.Buffer
	cw := NewConsoleWriter(WithOutput(&buf), WithNoColor())

	line := newTestLine("error", "Connection failed")
	if err := cw.Write(line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	want := "[2024-01-15 10:30:00] [app.log] ERROR: Connection failed\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConsoleWriter_Colors(t *testing.T) {
	tests := []struct {
		level     string
		wantColor string
	}{
		{"debug", colorDim},
		{"info", colorReset},
		{"warn", colorYellow},
		{"error", colorRed},
		{"fatal", colorRed + colorBold},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			cw := NewConsoleWriter(WithOutput(&buf))

			line := newTestLine(tt.level, "test message")
			if err := cw.Write(line); err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			got := buf.String()
			if !strings.HasPrefix(got, tt.wantColor) {
				t.Errorf("level %q: output doesn't start with expected color code", tt.level)
			}
			if !strings.HasSuffix(got, colorReset+"\n") {
				t.Errorf("level %q: output doesn't end with reset code", tt.level)
			}
		})
	}
}

func TestConsoleWriter_NoColor(t *testing.T) {
	var buf bytes.Buffer
	cw := NewConsoleWriter(WithOutput(&buf), WithNoColor())

	line := newTestLine("error", "test")
	if err := cw.Write(line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "\033[") {
		t.Errorf("no-color output contains ANSI escape codes: %q", got)
	}
}

func TestConsoleWriter_CustomTimestamp(t *testing.T) {
	var buf bytes.Buffer
	cw := NewConsoleWriter(WithOutput(&buf), WithNoColor(), WithTimestampFormat(time.RFC3339))

	line := newTestLine("info", "test")
	if err := cw.Write(line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "2024-01-15T10:30:00Z") {
		t.Errorf("custom timestamp not found in output: %q", got)
	}
}

// --- JSONWriter tests ---

func TestJSONWriter_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	line := newTestLine("error", "Connection failed")
	line.Fields["request_id"] = "abc-123"

	if err := jw.Write(line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed logline.LogLine
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, buf.String())
	}

	if parsed.Level != "error" {
		t.Errorf("Level = %q, want %q", parsed.Level, "error")
	}
	if parsed.Message != "Connection failed" {
		t.Errorf("Message = %q, want %q", parsed.Message, "Connection failed")
	}
	if parsed.Source != "app.log" {
		t.Errorf("Source = %q, want %q", parsed.Source, "app.log")
	}
}

func TestJSONWriter_NDJSON(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	_ = jw.Write(newTestLine("info", "line one"))
	_ = jw.Write(newTestLine("error", "line two"))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	for i, l := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(l), &parsed); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

// --- FileWriter tests ---

func TestFileWriter_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.log")

	fw, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}

	line := newTestLine("error", "test error")
	if err := fw.Write(line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "ERROR: test error") {
		t.Errorf("file content missing expected output: %q", got)
	}
	if !strings.Contains(got, "[app.log]") {
		t.Errorf("file content missing source: %q", got)
	}
}

func TestFileWriter_Appends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.log")

	fw, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}
	_ = fw.Write(newTestLine("info", "first"))
	_ = fw.Close()

	fw2, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter() second open error = %v", err)
	}
	_ = fw2.Write(newTestLine("error", "second"))
	_ = fw2.Close()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
}

func TestFileWriter_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secure.log")

	fw, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}
	_ = fw.Write(newTestLine("info", "test"))
	_ = fw.Close()

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := fi.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestFileWriter_InvalidPath(t *testing.T) {
	_, err := NewFileWriter("/nonexistent/dir/output.log")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

// --- MultiWriter tests ---

func TestMultiWriter_FansOut(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	mw := NewMultiWriter(
		NewConsoleWriter(WithOutput(&buf1), WithNoColor()),
		NewJSONWriter(&buf2),
	)

	line := newTestLine("error", "test")
	if err := mw.Write(line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if buf1.Len() == 0 {
		t.Error("console writer received no output")
	}
	if buf2.Len() == 0 {
		t.Error("json writer received no output")
	}
}

// --- FormatLogLine test ---

func TestFormatLogLine(t *testing.T) {
	line := newTestLine("warn", "Slow query")
	got := FormatLogLine(line)
	want := "[2024-01-15 10:30:00] [app.log] WARN: Slow query"
	if got != want {
		t.Errorf("FormatLogLine() = %q, want %q", got, want)
	}
}
