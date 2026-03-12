package cmd

import (
	"context"
	"fmt"
	"os"

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
) error {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n" + healthMonitor.Summary())
			return nil

		case err := <-errChan:
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		case raw, ok := <-logChan:
			if !ok {
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

			if err := writer.Write(parsed); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Output error: %v\n", err)
			}

			if apiServer != nil {
				apiServer.Hub().Broadcast(parsed)
			}
		}
	}
}
