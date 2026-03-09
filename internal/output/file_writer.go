package output

import (
	"compress/gzip"
	"fmt"
	"io"
	"logtailr/pkg/logline"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileWriter writes log lines to a file with optional rotation.
type FileWriter struct {
	path     string
	file     *os.File
	mu       sync.Mutex
	size     int64
	maxSize  int64
	maxAge   time.Duration
	compress bool
}

// FileOption configures the FileWriter.
type FileOption func(*FileWriter)

// WithMaxSize sets the maximum file size in bytes before rotation.
func WithMaxSize(bytes int64) FileOption {
	return func(fw *FileWriter) { fw.maxSize = bytes }
}

// WithMaxAge sets the maximum age of rotated files before cleanup.
func WithMaxAge(d time.Duration) FileOption {
	return func(fw *FileWriter) { fw.maxAge = d }
}

// WithCompress enables gzip compression of rotated files.
func WithCompress() FileOption {
	return func(fw *FileWriter) { fw.compress = true }
}

// NewFileWriter creates a FileWriter that appends to the given file path.
// The path is resolved to an absolute path and validated to prevent path traversal.
func NewFileWriter(path string, opts ...FileOption) (*FileWriter, error) {
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

	// Get current file size for rotation tracking
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	fw := &FileWriter{
		path: absPath,
		file: f,
		size: fi.Size(),
	}
	for _, opt := range opts {
		opt(fw)
	}

	return fw, nil
}

func (fw *FileWriter) Write(line *logline.LogLine) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	ts := line.Timestamp.Format(defaultTimestampFormat)
	level := strings.ToUpper(line.Level)
	formatted := fmt.Sprintf("[%s] [%s] %s: %s\n", ts, line.Source, level, line.Message)

	// Check if rotation is needed before writing
	if fw.maxSize > 0 && fw.size+int64(len(formatted)) > fw.maxSize {
		if err := fw.rotate(); err != nil {
			return fmt.Errorf("file rotation failed: %w", err)
		}
	}

	n, err := fw.file.WriteString(formatted)
	fw.size += int64(n)
	return err
}

func (fw *FileWriter) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.file != nil {
		return fw.file.Close()
	}
	return nil
}

// rotate closes the current file, renames it with a timestamp suffix,
// optionally compresses it, cleans up old files, and opens a new file.
// Must be called with fw.mu held.
func (fw *FileWriter) rotate() error {
	if err := fw.file.Close(); err != nil {
		return fmt.Errorf("close current file: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	rotatedPath := fw.path + "." + timestamp

	if err := os.Rename(fw.path, rotatedPath); err != nil {
		return fmt.Errorf("rename to rotated: %w", err)
	}

	if fw.compress {
		go compressFile(rotatedPath)
	}

	// Open a new file
	f, err := os.OpenFile(fw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open new file after rotation: %w", err)
	}
	fw.file = f
	fw.size = 0

	// Cleanup old rotated files
	if fw.maxAge > 0 {
		go fw.cleanupOldFiles()
	}

	return nil
}

// compressFile gzip-compresses a file and removes the original.
func compressFile(path string) {
	src, err := os.Open(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "compress: open %s: %v\n", path, err)
		return
	}

	dst, err := os.OpenFile(path+".gz", os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		_ = src.Close()
		_, _ = fmt.Fprintf(os.Stderr, "compress: create %s.gz: %v\n", path, err)
		return
	}

	gz := gzip.NewWriter(dst)

	if _, err := io.Copy(gz, src); err != nil {
		_ = gz.Close()
		_ = dst.Close()
		_ = src.Close()
		_, _ = fmt.Fprintf(os.Stderr, "compress: copy %s: %v\n", path, err)
		return
	}

	if err := gz.Close(); err != nil {
		_ = dst.Close()
		_ = src.Close()
		_, _ = fmt.Fprintf(os.Stderr, "compress: finalize %s: %v\n", path, err)
		return
	}
	_ = dst.Close()
	_ = src.Close()

	// Remove uncompressed original
	_ = os.Remove(path)
}

// cleanupOldFiles removes rotated files older than maxAge.
func (fw *FileWriter) cleanupOldFiles() {
	dir := filepath.Dir(fw.path)
	base := filepath.Base(fw.path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-fw.maxAge)

	// Collect rotated files
	type rotatedFile struct {
		path    string
		modTime time.Time
	}
	var candidates []rotatedFile

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, base+".") {
			continue
		}
		// Skip the current log file
		if name == base {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			candidates = append(candidates, rotatedFile{
				path:    filepath.Join(dir, name),
				modTime: info.ModTime(),
			})
		}
	}

	// Sort oldest first and remove
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.Before(candidates[j].modTime)
	})

	for _, c := range candidates {
		_ = os.Remove(c.path)
	}
}
