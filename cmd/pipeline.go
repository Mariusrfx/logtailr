package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"logtailr/internal/aggregator"
	"logtailr/internal/alert"
	"logtailr/internal/api"
	"logtailr/internal/filter"
	"logtailr/internal/health"
	"logtailr/internal/output"
	"logtailr/internal/parser"
	"logtailr/pkg/logline"
)

// runPipeline consumes log lines until ctx is cancelled, then keeps draining
// logChan/errChan until the tailer manager has stopped all tailers (stopped
// channel) and the channels are empty, so lines already emitted by tailers
// are not lost on shutdown.
func runPipeline(
	ctx context.Context,
	logChan <-chan *logline.LogLine,
	errChan <-chan error,
	regexFilter *filter.RegexFilter,
	writer output.Writer,
	healthMonitor *health.Monitor,
	apiServer *api.Server,
	alertEngine *alert.Engine,
	agg *aggregator.Aggregator,
	stopped <-chan struct{},
) error {
	var aggChan <-chan []*aggregator.AggregatedLine
	if agg != nil {
		aggChan = agg.Expired()
	}

	process := func(raw *logline.LogLine) {
		logParser := parser.New(raw.Source)
		parsed, err := logParser.Parse(raw.Message, parserFlag)
		if err != nil {
			parsed = raw
		} else {
			parsed.Source = raw.Source
		}

		if apiServer != nil {
			safeSource := api.SanitizeLabel(parsed.Source, 128)
			safeLevel := api.SanitizeLabel(parsed.Level, 16)
			apiServer.Metrics().LogsTotal.WithLabelValues(safeSource, safeLevel).Inc()
		}

		if alertEngine != nil {
			alertEngine.ProcessLine(parsed)
		}

		if !filter.ByLevel(parsed, level) {
			return
		}

		if !regexFilter.Match(parsed.Message) {
			return
		}

		if agg != nil {
			for _, r := range agg.Process(parsed) {
				writeAndBroadcast(r.Line, writer, apiServer)
			}
			return
		}

		writeAndBroadcast(parsed, writer, apiServer)
	}

	flushAgg := func() {
		if agg != nil {
			for _, r := range agg.Flush() {
				writeAndBroadcast(r.Line, writer, apiServer)
			}
			agg.Stop()
		}
	}

	draining := false
	for {
		if !draining {
			select {
			case <-ctx.Done():
				draining = true
				flushAgg()
				continue

			case expired := <-aggChan:
				for _, r := range expired {
					writeAndBroadcast(r.Line, writer, apiServer)
				}

			case err := <-errChan:
				slog.Error("source error", "error", err)

			case raw, ok := <-logChan:
				if !ok {
					flushAgg()
					fmt.Println("\n" + healthMonitor.Summary())
					return nil
				}
				process(raw)
			}
			continue
		}

		// Draining: tailers are being stopped; consume what is still in
		// flight, and once the manager confirms all tailers are stopped do a
		// final non-blocking sweep of both channels before exiting.
		select {
		case <-stopped:
			for {
				select {
				case raw := <-logChan:
					process(raw)
				case err := <-errChan:
					slog.Error("source error", "error", err)
				default:
					fmt.Println("\n" + healthMonitor.Summary())
					return nil
				}
			}

		case raw := <-logChan:
			process(raw)

		case err := <-errChan:
			slog.Error("source error", "error", err)
		}
	}
}

func writeAndBroadcast(line *logline.LogLine, writer output.Writer, apiServer *api.Server) {
	if err := writer.Write(line); err != nil {
		slog.Error("output write failed", "error", err)
	}
	if apiServer != nil {
		apiServer.Hub().Broadcast(line)
	}
}
