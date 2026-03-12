package output

import (
	"fmt"
	"logtailr/pkg/logline"
	"strings"
)

func FormatLogLine(line *logline.LogLine) string {
	ts := line.Timestamp.Format(defaultTimestampFormat)
	level := strings.ToUpper(line.Level)
	return fmt.Sprintf("[%s] [%s] %s: %s", ts, line.Source, level, line.Message)
}
