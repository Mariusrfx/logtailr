package output

import "logtailr/pkg/logline"

type Writer interface {
	Write(line *logline.LogLine) error
	Close() error
}

// StatsWriter is optionally implemented by writers that expose delivery stats.
type StatsWriter interface {
	SetStatsSink(func(failedBatches int64, pendingDocs int, droppedDocs int64))
}
