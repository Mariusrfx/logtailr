package output

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"logtailr/pkg/logline"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultBulkSize      = 500
	defaultFlushInterval = 5 * time.Second
	defaultMaxRetries    = 3
	defaultRetryBaseWait = 500 * time.Millisecond
	defaultHTTPTimeout   = 30 * time.Second
	maxBulkSize          = 10000
	maxFlushInterval     = 60 * time.Second
	maxBackoffWait       = 30 * time.Second
	maxResponseBodyRead  = 1 << 20 // 1MB
)

type OpenSearchConfig struct {
	Hosts         []string `mapstructure:"hosts"`
	Index         string   `mapstructure:"index"`
	Username      string   `mapstructure:"username"`
	Password      string   `mapstructure:"password"`
	BulkSize      int      `mapstructure:"bulk_size"`
	FlushInterval string   `mapstructure:"flush_interval"`
	TLSSkipVerify bool     `mapstructure:"tls_skip_verify"`
	MaxRetries    int      `mapstructure:"max_retries"`
	TemplateName  string   `mapstructure:"template_name"`
	DashboardsURL string   `mapstructure:"dashboards_url"`
}

type OpenSearchWriter struct {
	client        *http.Client
	hosts         []string
	index         string
	username      string
	password      string
	bulkSize      int
	flushInterval time.Duration
	maxRetries    int
	templateName  string
	dashboardsURL string

	buffer []json.RawMessage
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func NewOpenSearchWriter(cfg OpenSearchConfig) (*OpenSearchWriter, error) {
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("opensearch: at least one host is required")
	}
	if cfg.Index == "" {
		return nil, fmt.Errorf("opensearch: index is required")
	}

	bulkSize := cfg.BulkSize
	if bulkSize <= 0 {
		bulkSize = defaultBulkSize
	}
	if bulkSize > maxBulkSize {
		return nil, fmt.Errorf("opensearch: bulk_size must be <= %d", maxBulkSize)
	}

	flushInterval := defaultFlushInterval
	if cfg.FlushInterval != "" {
		d, err := time.ParseDuration(cfg.FlushInterval)
		if err != nil {
			return nil, fmt.Errorf("opensearch: invalid flush_interval: %w", err)
		}
		if d > maxFlushInterval {
			return nil, fmt.Errorf("opensearch: flush_interval must be <= %s", maxFlushInterval)
		}
		if d > 0 {
			flushInterval = d
		}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if cfg.TLSSkipVerify {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // user-configured
	}
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 5,
		TLSClientConfig:     tlsCfg,
	}

	ctx, cancel := context.WithCancel(context.Background())

	templateName := cfg.TemplateName
	if templateName == "" {
		templateName = "logtailr"
	}

	ow := &OpenSearchWriter{
		client: &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: transport,
		},
		hosts:         cfg.Hosts,
		index:         cfg.Index,
		username:      cfg.Username,
		password:      cfg.Password,
		bulkSize:      bulkSize,
		flushInterval: flushInterval,
		maxRetries:    maxRetries,
		templateName:  templateName,
		dashboardsURL: cfg.DashboardsURL,
		buffer:        make([]json.RawMessage, 0, bulkSize),
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	if err := ow.ensureIndexTemplate(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "opensearch: failed to create index template: %v\n", err)
	}

	if ow.dashboardsURL != "" {
		if err := ow.ensureIndexPattern(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "opensearch: failed to create dashboards index pattern: %v\n", err)
		}
	}

	go ow.flushLoop()

	return ow, nil
}

func (ow *OpenSearchWriter) Write(line *logline.LogLine) error {
	doc, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("opensearch: failed to marshal log line: %w", err)
	}

	ow.mu.Lock()
	ow.buffer = append(ow.buffer, doc)
	shouldFlush := len(ow.buffer) >= ow.bulkSize
	ow.mu.Unlock()

	if shouldFlush {
		return ow.flush()
	}
	return nil
}

func (ow *OpenSearchWriter) Close() error {
	ow.cancel()
	<-ow.done
	return ow.finalFlush()
}

func (ow *OpenSearchWriter) flushLoop() {
	defer close(ow.done)

	ticker := time.NewTicker(ow.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ow.ctx.Done():
			return
		case <-ticker.C:
			if err := ow.flush(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "opensearch flush error: %v\n", err)
			}
		}
	}
}

func (ow *OpenSearchWriter) finalFlush() error {
	ow.mu.Lock()
	if len(ow.buffer) == 0 {
		ow.mu.Unlock()
		return nil
	}
	batch := ow.buffer
	ow.buffer = nil
	ow.mu.Unlock()

	return ow.sendBulkWithContext(context.Background(), batch)
}

func (ow *OpenSearchWriter) flush() error {
	ow.mu.Lock()
	if len(ow.buffer) == 0 {
		ow.mu.Unlock()
		return nil
	}
	batch := ow.buffer
	ow.buffer = make([]json.RawMessage, 0, ow.bulkSize)
	ow.mu.Unlock()

	return ow.sendBulkWithContext(ow.ctx, batch)
}

func (ow *OpenSearchWriter) sendBulkWithContext(ctx context.Context, docs []json.RawMessage) error {
	body := ow.buildBulkBody(docs)

	var lastErr error
	for attempt := range ow.maxRetries {
		if attempt > 0 {
			wait := time.Duration(math.Pow(2, float64(attempt-1))) * defaultRetryBaseWait
			if wait > maxBackoffWait {
				wait = maxBackoffWait
			}
			time.Sleep(wait)
		}

		err := ow.doRequest(ctx, body)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("opensearch: bulk insert failed after %d retries: %w", ow.maxRetries, lastErr)
}

func (ow *OpenSearchWriter) buildBulkBody(docs []json.RawMessage) []byte {
	var buf bytes.Buffer
	index := ow.resolveIndex()

	action := fmt.Sprintf(`{"index":{"_index":"%s"}}`, index)
	for _, doc := range docs {
		buf.WriteString(action)
		buf.WriteByte('\n')
		buf.Write(doc)
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

func (ow *OpenSearchWriter) resolveIndex() string {
	now := time.Now().UTC()
	index := ow.index
	index = strings.ReplaceAll(index, "%{+YYYY.MM.dd}", now.Format("2006.01.02"))
	index = strings.ReplaceAll(index, "%{+YYYY.MM}", now.Format("2006.01"))
	index = strings.ReplaceAll(index, "%{+YYYY}", now.Format("2006"))
	return index
}

func (ow *OpenSearchWriter) doRequest(ctx context.Context, body []byte) error {
	host := ow.hosts[time.Now().UnixNano()%int64(len(ow.hosts))]
	url := strings.TrimRight(host, "/") + "/_bulk"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	if ow.username != "" {
		req.SetBasicAuth(ow.username, ow.password)
	}

	resp, err := ow.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limitedBody := io.LimitReader(resp.Body, maxResponseBodyRead)

	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, limitedBody)
		return fmt.Errorf("opensearch bulk insert failed: HTTP %d", resp.StatusCode)
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
	}
	if err := json.NewDecoder(limitedBody).Decode(&bulkResp); err == nil && bulkResp.Errors {
		return fmt.Errorf("opensearch bulk response contains partial errors")
	}

	return nil
}

func (ow *OpenSearchWriter) ensureIndexTemplate() error {
	indexPattern := ow.index
	for _, token := range []string{"%{+YYYY.MM.dd}", "%{+YYYY.MM}", "%{+YYYY}"} {
		indexPattern = strings.ReplaceAll(indexPattern, token, "*")
	}

	template := map[string]any{
		"index_patterns": []string{indexPattern},
		"template": map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"timestamp": map[string]any{"type": "date"},
					"level":     map[string]any{"type": "keyword"},
					"message":   map[string]any{"type": "text"},
					"source":    map[string]any{"type": "keyword"},
					"fields":    map[string]any{"type": "object", "enabled": true},
				},
			},
		},
		"priority": 100,
	}

	body, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	host := ow.hosts[0]
	url := strings.TrimRight(host, "/") + "/_index_template/" + ow.templateName

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ow.username != "" {
		req.SetBasicAuth(ow.username, ow.password)
	}

	resp, err := ow.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyRead))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d creating index template", resp.StatusCode)
	}

	return nil
}

func (ow *OpenSearchWriter) ensureIndexPattern() error {
	indexPattern := ow.index
	for _, token := range []string{"%{+YYYY.MM.dd}", "%{+YYYY.MM}", "%{+YYYY}"} {
		indexPattern = strings.ReplaceAll(indexPattern, token, "*")
	}

	payload := map[string]any{
		"attributes": map[string]any{
			"title":         indexPattern,
			"timeFieldName": "timestamp",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal index pattern: %w", err)
	}

	url := strings.TrimRight(ow.dashboardsURL, "/") + "/api/saved_objects/index-pattern/" + ow.templateName

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("osd-xsrf", "true")

	resp, err := ow.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyRead))

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("HTTP %d creating index pattern", resp.StatusCode)
	}

	return nil
}
