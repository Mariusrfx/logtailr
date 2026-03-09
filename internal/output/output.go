package output

import "logtailr/pkg/logline"

// Writer is the interface for all output destinations.
type Writer interface {
	Write(line *logline.LogLine) error
	Close() error
}
