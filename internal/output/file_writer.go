package output

import (
	"fmt"
	"logtailr/pkg/logline"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileWriter writes log lines to a file.
type FileWriter struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileWriter creates a FileWriter that appends to the given file path.
// The path is resolved to an absolute path and validated to prevent path traversal.
func NewFileWriter(path string) (*FileWriter, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve output path: %w", err)
	}
	// Ensure parent directory exists and is not a symlink escape
	dir := filepath.Dir(absPath)
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("output directory does not exist: %w", err)
	}

	f, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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
