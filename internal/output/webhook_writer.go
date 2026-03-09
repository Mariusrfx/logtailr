package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"logtailr/pkg/logline"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultWebhookBatchSize    = 10
	defaultWebhookBatchTimeout = 30 * time.Second
	defaultWebhookHTTPTimeout  = 15 * time.Second
	maxWebhookBatchSize        = 100
	maxWebhookBatchTimeout     = 120 * time.Second
)

// WebhookConfig holds the configuration for the webhook writer.
type WebhookConfig struct {
	URL          string `mapstructure:"url"`
	MinLevel     string `mapstructure:"min_level"`
	BatchSize    int    `mapstructure:"batch_size"`
	BatchTimeout string `mapstructure:"batch_timeout"`
}

// webhookPayload is the JSON structure sent to the webhook endpoint.
type webhookPayload struct {
	Text   string           `json:"text"`
	Logs   []webhookLogItem `json:"logs"`
	Count  int              `json:"count"`
	SentAt string           `json:"sent_at"`
}

type webhookLogItem struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

// WebhookWriter sends batched log lines to an HTTP webhook endpoint.
type WebhookWriter struct {
	client       *http.Client
	url          string
	minLevel     int
	batchSize    int
	batchTimeout time.Duration

	buffer []webhookLogItem
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewWebhookWriter creates a new WebhookWriter from config.
func NewWebhookWriter(cfg WebhookConfig) (*WebhookWriter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("webhook: url is required")
	}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return nil, fmt.Errorf("webhook: url must start with http:// or https://")
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultWebhookBatchSize
	}
	if batchSize > maxWebhookBatchSize {
		return nil, fmt.Errorf("webhook: batch_size must be <= %d", maxWebhookBatchSize)
	}

	batchTimeout := defaultWebhookBatchTimeout
	if cfg.BatchTimeout != "" {
		d, err := time.ParseDuration(cfg.BatchTimeout)
		if err != nil {
			return nil, fmt.Errorf("webhook: invalid batch_timeout: %w", err)
		}
		if d > maxWebhookBatchTimeout {
			return nil, fmt.Errorf("webhook: batch_timeout must be <= %s", maxWebhookBatchTimeout)
		}
		if d > 0 {
			batchTimeout = d
		}
	}

	minLevel := 0
	if cfg.MinLevel != "" {
		lvl, ok := logline.LogLevels[strings.ToLower(cfg.MinLevel)]
		if !ok {
			return nil, fmt.Errorf("webhook: invalid min_level %q", cfg.MinLevel)
		}
		minLevel = lvl
	}

	ctx, cancel := context.WithCancel(context.Background())

	ww := &WebhookWriter{
		client: &http.Client{
			Timeout: defaultWebhookHTTPTimeout,
		},
		url:          cfg.URL,
		minLevel:     minLevel,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		buffer:       make([]webhookLogItem, 0, batchSize),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
	}

	go ww.flushLoop()

	return ww, nil
}

// Write adds a log line to the batch. Flushes when batch is full.
func (ww *WebhookWriter) Write(line *logline.LogLine) error {
	// Filter by min level
	lineLevel := logline.LogLevels[strings.ToLower(line.Level)]
	if lineLevel < ww.minLevel {
		return nil
	}

	item := webhookLogItem{
		Timestamp: line.Timestamp.Format(time.RFC3339),
		Level:     strings.ToUpper(line.Level),
		Message:   line.Message,
		Source:    line.Source,
	}

	ww.mu.Lock()
	ww.buffer = append(ww.buffer, item)
	shouldFlush := len(ww.buffer) >= ww.batchSize
	ww.mu.Unlock()

	if shouldFlush {
		return ww.flush()
	}
	return nil
}

// Close flushes remaining logs and shuts down the writer.
func (ww *WebhookWriter) Close() error {
	ww.cancel()
	<-ww.done
	return ww.finalFlush()
}

func (ww *WebhookWriter) flushLoop() {
	defer close(ww.done)

	ticker := time.NewTicker(ww.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ww.ctx.Done():
			return
		case <-ticker.C:
			if err := ww.flush(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "webhook flush error: %v\n", err)
			}
		}
	}
}

// finalFlush sends remaining items using a background context (for use after cancel).
func (ww *WebhookWriter) finalFlush() error {
	ww.mu.Lock()
	if len(ww.buffer) == 0 {
		ww.mu.Unlock()
		return nil
	}
	batch := ww.buffer
	ww.buffer = nil
	ww.mu.Unlock()

	return ww.sendWithContext(context.Background(), batch)
}

func (ww *WebhookWriter) flush() error {
	ww.mu.Lock()
	if len(ww.buffer) == 0 {
		ww.mu.Unlock()
		return nil
	}
	batch := ww.buffer
	ww.buffer = make([]webhookLogItem, 0, ww.batchSize)
	ww.mu.Unlock()

	return ww.sendWithContext(ww.ctx, batch)
}

func (ww *WebhookWriter) sendWithContext(ctx context.Context, items []webhookLogItem) error {
	// Build summary text
	levelCounts := make(map[string]int)
	for _, item := range items {
		levelCounts[item.Level]++
	}

	var parts []string
	for lvl, count := range levelCounts {
		parts = append(parts, fmt.Sprintf("%d %s", count, lvl))
	}

	payload := webhookPayload{
		Text:   fmt.Sprintf("Logtailr: %d log(s) — %s", len(items), strings.Join(parts, ", ")),
		Logs:   items,
		Count:  len(items),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ww.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ww.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20)) // limit to 1MB

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: request failed with HTTP %d", resp.StatusCode)
	}

	return nil
}
