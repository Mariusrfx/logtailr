package cmd

import (
	"context"
	"fmt"
	"os"

	"logtailr/internal/aggregator"
	"logtailr/internal/alert"
	"logtailr/internal/api"
	"logtailr/internal/filter"
	"logtailr/internal/health"
	"logtailr/internal/output"
	"logtailr/internal/parser"
	"logtailr/pkg/logline"
)

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
) error {
	var aggChan <-chan []*aggregator.AggregatedLine
	if agg != nil {
		aggChan = agg.Expired()
	}

	for {
		select {
		case <-ctx.Done():
			if agg != nil {
				for _, r := range agg.Flush() {
					writeAndBroadcast(r.Line, writer, apiServer)
				}
				agg.Stop()
			}
			fmt.Println("\n" + healthMonitor.Summary())
			return nil

		case expired := <-aggChan:
			for _, r := range expired {
				writeAndBroadcast(r.Line, writer, apiServer)
			}

		case err := <-errChan:
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		case raw, ok := <-logChan:
			if !ok {
				if agg != nil {
					for _, r := range agg.Flush() {
						writeAndBroadcast(r.Line, writer, apiServer)
					}
					agg.Stop()
				}
				fmt.Println("\n" + healthMonitor.Summary())
				return nil
			}

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
				continue
			}

			if !regexFilter.Match(parsed.Message) {
				continue
			}

			if agg != nil {
				results := agg.Process(parsed)
				for _, r := range results {
					writeAndBroadcast(r.Line, writer, apiServer)
				}
				continue
			}

			writeAndBroadcast(parsed, writer, apiServer)
		}
	}
}

func writeAndBroadcast(line *logline.LogLine, writer output.Writer, apiServer *api.Server) {
	if err := writer.Write(line); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Output error: %v\n", err)
	}
	if apiServer != nil {
		apiServer.Hub().Broadcast(line)
	}
}
