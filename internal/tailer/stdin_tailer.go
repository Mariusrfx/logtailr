package tailer

import (
	"bufio"
	"context"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os"
	"time"
)

type StdinTailer struct {
	BaseTailer
	cancel context.CancelFunc
}

func NewStdinTailer(healthMonitor *health.Monitor) *StdinTailer {
	name := "stdin"
	st := &StdinTailer{
		BaseTailer: BaseTailer{
			SourceName:    name,
			HealthMonitor: healthMonitor,
		},
	}

	if healthMonitor != nil {
		healthMonitor.RegisterSource(name)
	}

	return st
}

func (st *StdinTailer) Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	ctx, st.cancel = context.WithCancel(ctx)

	go st.run(ctx, out, errChan)
}

func (st *StdinTailer) Stop() error {
	if st.cancel != nil {
		st.cancel()
	}
	st.ReportStopped()
	return nil
}

func (st *StdinTailer) run(ctx context.Context, out chan<- *logline.LogLine, _ chan<- error) {
	st.ReportHealthy()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, readBufferSize), maxLineSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		ll := &logline.LogLine{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   line,
			Source:    st.SourceName,
			Fields:    make(map[string]interface{}),
		}

		select {
		case out <- ll:
		case <-ctx.Done():
			return
		}
	}

	// stdin closed — normal EOF, not an error
}
