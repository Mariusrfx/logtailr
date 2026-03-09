package cmd

import (
	"fmt"
	"os"

	"logtailr/internal/config"
	"logtailr/internal/output"
)

func createWriter(outputsCfg *config.OutputsConfig) (output.Writer, error) {
	var primary output.Writer
	switch outputFlag {
	case "json":
		primary = output.NewJSONWriter(os.Stdout)
	case "file":
		fw, err := output.NewFileWriter(outputPath)
		if err != nil {
			return nil, err
		}
		primary = fw
	default:
		primary = output.NewConsoleWriter()
	}

	if outputsCfg == nil {
		return primary, nil
	}

	// Collect additional writers from outputs config
	writers := []output.Writer{primary}

	if outputsCfg.OpenSearch != nil && outputsCfg.OpenSearch.Enabled {
		osCfg := outputsCfg.OpenSearch
		ow, err := output.NewOpenSearchWriter(output.OpenSearchConfig{
			Hosts:         osCfg.Hosts,
			Index:         osCfg.Index,
			Username:      osCfg.Username,
			Password:      osCfg.Password,
			BulkSize:      osCfg.BulkSize,
			FlushInterval: osCfg.FlushInterval,
			TLSSkipVerify: osCfg.TLSSkipVerify,
			MaxRetries:    osCfg.MaxRetries,
		})
		if err != nil {
			return nil, fmt.Errorf("opensearch output: %w", err)
		}
		writers = append(writers, ow)
	}

	if outputsCfg.Webhook != nil && outputsCfg.Webhook.Enabled {
		wh := outputsCfg.Webhook
		ww, err := output.NewWebhookWriter(output.WebhookConfig{
			URL:          wh.URL,
			MinLevel:     wh.MinLevel,
			BatchSize:    wh.BatchSize,
			BatchTimeout: wh.BatchTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("webhook output: %w", err)
		}
		writers = append(writers, ww)
	}

	if len(writers) == 1 {
		return primary, nil
	}
	return output.NewMultiWriter(writers...), nil
}
