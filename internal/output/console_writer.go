package output

import (
	"fmt"
	"io"
	"logtailr/pkg/logline"
	"os"
	"strings"
	"sync"
)

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

type ConsoleWriter struct {
	out      io.Writer
	noColor  bool
	mu       sync.Mutex
	tsFormat string
}

type ConsoleOption func(*ConsoleWriter)

func WithNoColor() ConsoleOption {
	return func(cw *ConsoleWriter) { cw.noColor = true }
}

func WithOutput(w io.Writer) ConsoleOption {
	return func(cw *ConsoleWriter) { cw.out = w }
}

func WithTimestampFormat(format string) ConsoleOption {
	return func(cw *ConsoleWriter) { cw.tsFormat = format }
}

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
