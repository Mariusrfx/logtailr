package output

import "logtailr/pkg/logline"

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
