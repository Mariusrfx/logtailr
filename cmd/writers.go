package cmd

import (
	"fmt"
	"os"
	"time"

	"logtailr/internal/config"
	"logtailr/internal/output"
)

func createWriter(outputsCfg *config.OutputsConfig) (output.Writer, error) {
	var primary output.Writer
	switch outputFlag {
	case "json":
		primary = output.NewJSONWriter(os.Stdout)
	case "file":
		opts := fileOptsFromConfig(outputsCfg)
		fw, err := output.NewFileWriter(outputPath, opts...)
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

	// File output from config (separate from --output=file flag)
	if outputsCfg.File != nil && outputsCfg.File.Path != "" && outputFlag != "file" {
		opts := fileOptsFromOutputConfig(outputsCfg.File)
		fw, err := output.NewFileWriter(outputsCfg.File.Path, opts...)
		if err != nil {
			return nil, fmt.Errorf("file output: %w", err)
		}
		writers = append(writers, fw)
	}

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

// fileOptsFromConfig extracts file rotation options from the outputs config
// when using --output=file flag mode.
func fileOptsFromConfig(outputsCfg *config.OutputsConfig) []output.FileOption {
	if outputsCfg == nil || outputsCfg.File == nil {
		return nil
	}
	return fileOptsFromOutputConfig(outputsCfg.File)
}

// fileOptsFromOutputConfig converts FileOutputConfig to FileOption slice.
func fileOptsFromOutputConfig(fc *config.FileOutputConfig) []output.FileOption {
	var opts []output.FileOption

	if fc.MaxSize != "" {
		if bytes, err := config.ParseByteSize(fc.MaxSize); err == nil {
			opts = append(opts, output.WithMaxSize(bytes))
		}
	}
	if fc.MaxAge != "" {
		if d, err := time.ParseDuration(fc.MaxAge); err == nil {
			opts = append(opts, output.WithMaxAge(d))
		}
	}
	if fc.Compress {
		opts = append(opts, output.WithCompress())
	}

	return opts
}
