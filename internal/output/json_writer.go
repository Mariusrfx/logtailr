package output

import (
	"encoding/json"
	"fmt"
	"io"
	"logtailr/pkg/logline"
	"sync"
)

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
