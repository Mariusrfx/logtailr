package output

import "logtailr/pkg/logline"

type Writer interface {
	Write(line *logline.LogLine) error
	Close() error
}
