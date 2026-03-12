package output

import "logtailr/pkg/logline"

type MultiWriter struct {
	writers []Writer
}

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
