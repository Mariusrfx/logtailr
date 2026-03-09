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

// --- FileWriter rotation tests ---

func TestFileWriter_RotateBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	// MaxSize of 100 bytes — each line is ~55 bytes, so 2nd line triggers rotation
	fw, err := NewFileWriter(path, WithMaxSize(100))
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}

	if err := fw.Write(newTestLine("info", "first line of log output")); err != nil {
		t.Fatalf("Write(1) error = %v", err)
	}
	if err := fw.Write(newTestLine("error", "second line triggers rotation")); err != nil {
		t.Fatalf("Write(2) error = %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// The current file should contain only the second line
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read current file: %v", err)
	}
	if !strings.Contains(string(content), "second line") {
		t.Errorf("current file should have second line, got: %q", string(content))
	}
	if strings.Contains(string(content), "first line") {
		t.Errorf("current file should NOT have first line after rotation")
	}

	// A rotated file should exist
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	rotatedCount := 0
	for _, e := range entries {
		if e.Name() != "app.log" {
			rotatedCount++
			rotatedContent, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read rotated file: %v", err)
			}
			if !strings.Contains(string(rotatedContent), "first line") {
				t.Errorf("rotated file should have first line, got: %q", string(rotatedContent))
			}
		}
	}
	if rotatedCount == 0 {
		t.Error("expected at least one rotated file")
	}
}

func TestFileWriter_RotatedFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.log")

	fw, err := NewFileWriter(path, WithMaxSize(50))
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}

	// Write enough to trigger rotation
	if err := fw.Write(newTestLine("info", "a]line that is long enough to exceed limit")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := fw.Write(newTestLine("info", "post rotation line")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	// Should have original + at least 1 rotated
	if len(entries) < 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("expected at least 2 files, got %d: %v", len(entries), names)
	}

	// Rotated file should have a timestamp pattern
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "output.log.") {
			// Verify timestamp format in name: output.log.YYYY-MM-DDTHH-MM-SS
			suffix := strings.TrimPrefix(e.Name(), "output.log.")
			if len(suffix) < 19 {
				t.Errorf("rotated file name has unexpected format: %q", e.Name())
			}
			return
		}
	}
	t.Error("no rotated file with timestamp suffix found")
}

func TestFileWriter_Compress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	fw, err := NewFileWriter(path, WithMaxSize(50), WithCompress())
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}

	if err := fw.Write(newTestLine("info", "line long enough to trigger rotation")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := fw.Write(newTestLine("info", "after rotation")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Give compression goroutine time to finish
	time.Sleep(500 * time.Millisecond)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			// Verify it's a valid gzip file
			info, err := e.Info()
			if err != nil {
				t.Fatalf("stat .gz file: %v", err)
			}
			if info.Size() == 0 {
				t.Error(".gz file is empty")
			}
			return
		}
	}
	t.Error("no .gz compressed file found")
}

func TestFileWriter_MaxAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	// Create a fake old rotated file
	oldFile := filepath.Join(dir, "app.log.2020-01-01T00-00-00")
	if err := os.WriteFile(oldFile, []byte("old data"), 0600); err != nil {
		t.Fatalf("create old file: %v", err)
	}
	// Set its mod time to the past
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// maxAge of 1 hour — the old file should be cleaned up after rotation
	fw, err := NewFileWriter(path, WithMaxSize(50), WithMaxAge(1*time.Hour))
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}

	// Trigger rotation
	if err := fw.Write(newTestLine("info", "line long enough to trigger rotation here")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := fw.Write(newTestLine("info", "after rotation")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Give cleanup goroutine time to run
	time.Sleep(500 * time.Millisecond)

	// Old file should be deleted
	if _, err := os.Stat(oldFile); err == nil {
		t.Error("old rotated file should have been deleted by maxAge cleanup")
	}
}

func TestFileWriter_NoRotationWithoutMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	fw, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}

	for range 10 {
		if err := fw.Write(newTestLine("info", "repeated line")); err != nil {
			t.Fatalf("Write error = %v", err)
		}
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file (no rotation), got %d", len(entries))
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
