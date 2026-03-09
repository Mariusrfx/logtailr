package output

import (
	"encoding/json"
	"fmt"
	"io"
	"logtailr/pkg/logline"
	"os"
	"strings"
	"sync"
)

// Writer is the interface for all output destinations.
type Writer interface {
	Write(line *logline.LogLine) error
	Close() error
}

// ANSI color codes for log levels
const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

var levelColors = map[string]string{
	"debug": colorDim,
	"info":  colorReset,
	"warn":  colorYellow,
	"error": colorRed,
	"fatal": colorRed + colorBold,
}

const defaultTimestampFormat = "2006-01-02 15:04:05"

// --- ConsoleWriter ---

// ConsoleWriter writes log lines to stdout with colors.
type ConsoleWriter struct {
	out      io.Writer
	noColor  bool
	mu       sync.Mutex
	tsFormat string
}

// ConsoleOption configures the ConsoleWriter.
type ConsoleOption func(*ConsoleWriter)

// WithNoColor disables ANSI color output.
func WithNoColor() ConsoleOption {
	return func(cw *ConsoleWriter) { cw.noColor = true }
}

// WithOutput sets a custom writer (useful for testing).
func WithOutput(w io.Writer) ConsoleOption {
	return func(cw *ConsoleWriter) { cw.out = w }
}

// WithTimestampFormat sets a custom timestamp format.
func WithTimestampFormat(format string) ConsoleOption {
	return func(cw *ConsoleWriter) { cw.tsFormat = format }
}

// NewConsoleWriter creates a ConsoleWriter that writes colored output to stdout.
func NewConsoleWriter(opts ...ConsoleOption) *ConsoleWriter {
	cw := &ConsoleWriter{
		out:      os.Stdout,
		tsFormat: defaultTimestampFormat,
	}
	for _, opt := range opts {
		opt(cw)
	}
	return cw
}

func (cw *ConsoleWriter) Write(line *logline.LogLine) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	ts := line.Timestamp.Format(cw.tsFormat)
	level := strings.ToUpper(line.Level)

	var formatted string
	if cw.noColor {
		formatted = fmt.Sprintf("[%s] [%s] %s: %s\n", ts, line.Source, level, line.Message)
	} else {
		color := levelColors[strings.ToLower(line.Level)]
		if color == "" {
			color = colorReset
		}
		formatted = fmt.Sprintf("%s[%s] [%s] %s: %s%s\n", color, ts, line.Source, level, line.Message, colorReset)
	}

	_, err := fmt.Fprint(cw.out, formatted)
	return err
}

func (cw *ConsoleWriter) Close() error {
	return nil
}

// --- JSONWriter ---

// JSONWriter writes log lines as JSON objects, one per line (NDJSON).
type JSONWriter struct {
	out io.Writer
	mu  sync.Mutex
}

// NewJSONWriter creates a JSONWriter that writes to the given writer.
func NewJSONWriter(out io.Writer) *JSONWriter {
	return &JSONWriter{out: out}
}

func (jw *JSONWriter) Write(line *logline.LogLine) error {
	jw.mu.Lock()
	defer jw.mu.Unlock()

	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("failed to marshal log line: %w", err)
	}

	data = append(data, '\n')
	_, err = jw.out.Write(data)
	return err
}

func (jw *JSONWriter) Close() error {
	return nil
}

// --- FileWriter ---

// FileWriter writes log lines to a file.
type FileWriter struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileWriter creates a FileWriter that appends to the given file path.
func NewFileWriter(path string) (*FileWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %w", err)
	}
	return &FileWriter{file: f}, nil
}

func (fw *FileWriter) Write(line *logline.LogLine) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	ts := line.Timestamp.Format(defaultTimestampFormat)
	level := strings.ToUpper(line.Level)
	formatted := fmt.Sprintf("[%s] [%s] %s: %s\n", ts, line.Source, level, line.Message)

	_, err := fw.file.WriteString(formatted)
	return err
}

func (fw *FileWriter) Close() error {
	if fw.file != nil {
		return fw.file.Close()
	}
	return nil
}

// --- MultiWriter ---

// MultiWriter fans out log lines to multiple writers.
type MultiWriter struct {
	writers []Writer
}

// NewMultiWriter creates a writer that sends to all provided writers.
func NewMultiWriter(writers ...Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) Write(line *logline.LogLine) error {
	for _, w := range mw.writers {
		if err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

func (mw *MultiWriter) Close() error {
	var firstErr error
	for _, w := range mw.writers {
		if err := w.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// FormatLogLine returns a plain text formatted line (for reuse).
func FormatLogLine(line *logline.LogLine) string {
	ts := line.Timestamp.Format(defaultTimestampFormat)
	level := strings.ToUpper(line.Level)
	return fmt.Sprintf("[%s] [%s] %s: %s", ts, line.Source, level, line.Message)
}
