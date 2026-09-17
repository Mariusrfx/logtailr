package output

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"logtailr/internal/safego"
	"logtailr/internal/ssrf"
	"logtailr/pkg/logline"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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
	shutdownFlushTimeout = 30 * time.Second
)

// maxPendingDocs bounds the in-memory retry queue (var so tests can shrink it).
var maxPendingDocs = 10000

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
	// AllowLocal disables SSRF protection (redirect re-validation) for local
	// development. Set programmatically, never from user config.
	AllowLocal bool
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

	buffer  []json.RawMessage
	pending []json.RawMessage // docs whose last bulk failed, re-sent on later flushes
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}

	failedBatches  atomic.Int64
	pendingDropped atomic.Int64
	statsSink      atomic.Pointer[func(failedBatches int64, pendingDocs int, droppedDocs int64)]
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
			Timeout:       defaultHTTPTimeout,
			Transport:     transport,
			CheckRedirect: ssrf.RedirectGuard(cfg.AllowLocal),
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

	// Run in the background so a slow/unreachable host cannot stall writer
	// creation (which happens in the hot-reload path).
	safego.Go("opensearch-ensure", func() {
		if err := ow.ensureIndexTemplate(); err != nil {
			slog.Error("opensearch: failed to create index template", "error", err)
		}
		if ow.dashboardsURL != "" {
			if err := ow.ensureIndexPattern(); err != nil {
				slog.Error("opensearch: failed to create dashboards index pattern", "error", err)
			}
		}
	}, nil)

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
	ctx, cancel := context.WithTimeout(context.Background(), shutdownFlushTimeout)
	defer cancel()
	return ow.finalFlush(ctx)
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
				slog.Error("opensearch flush error", "error", err)
			}
		}
	}
}

// finalFlush delivers whatever is left (retry queue + buffer) at shutdown.
// Failure means a bounded, counted loss — it is reported, never silent.
func (ow *OpenSearchWriter) finalFlush(ctx context.Context) error {
	ow.mu.Lock()
	pending := ow.pending
	ow.pending = nil
	batch := ow.buffer
	ow.buffer = nil
	ow.mu.Unlock()

	docs := append(append([]json.RawMessage{}, pending...), batch...)
	if len(docs) == 0 {
		return nil
	}

	failed, err := ow.sendBulkWithContext(ctx, docs)
	if len(failed) > 0 {
		ow.pendingDropped.Add(int64(len(failed)))
		ow.emitStats()
		return fmt.Errorf("opensearch: final flush failed, %d docs not delivered: %w", len(failed), err)
	}
	return err
}

// flush first re-sends the retry queue; only when it is empty (or recovered)
// does the current batch go out. Failed docs are re-queued, never discarded.
func (ow *OpenSearchWriter) flush() error {
	ow.mu.Lock()
	pending := ow.pending
	ow.pending = nil
	batch := ow.buffer
	ow.buffer = make([]json.RawMessage, 0, ow.bulkSize)
	ow.mu.Unlock()

	if len(pending) > 0 {
		failed, err := ow.sendBulkWithContext(ow.ctx, pending)
		if len(failed) > 0 {
			ow.countFailedBatch(err)
			ow.enqueuePending(failed)
			if len(batch) > 0 {
				ow.enqueuePending(batch)
			}
			return err
		}
		if len(batch) == 0 {
			return nil
		}
	}

	failed, err := ow.sendBulkWithContext(ow.ctx, batch)
	if len(failed) > 0 {
		ow.countFailedBatch(err)
		ow.enqueuePending(failed)
		return err
	}
	return err
}

// sendBulkWithContext retries per maxRetries (whole set on total failure,
// failed subset on per-item errors); returns the undelivered docs.
func (ow *OpenSearchWriter) sendBulkWithContext(ctx context.Context, docs []json.RawMessage) ([]json.RawMessage, error) {
	current := docs
	var lastErr error

	for attempt := range ow.maxRetries {
		if attempt > 0 {
			wait := time.Duration(math.Pow(2, float64(attempt-1))) * defaultRetryBaseWait
			if wait > maxBackoffWait {
				wait = maxBackoffWait
			}
			select {
			case <-ctx.Done():
				return current, fmt.Errorf("opensearch: bulk insert cancelled: %w", ctx.Err())
			case <-time.After(wait):
			}
		}

		body := ow.buildBulkBody(current)
		failed, err := ow.doRequest(ctx, body, current)
		if err == nil {
			if len(failed) == 0 {
				return nil, nil
			}
			lastErr = fmt.Errorf("opensearch: %d doc(s) failed in bulk response", len(failed))
			current = failed
			continue
		}
		lastErr = err
	}

	return current, fmt.Errorf("opensearch: bulk insert failed after %d retries: %w", ow.maxRetries, lastErr)
}

func (ow *OpenSearchWriter) enqueuePending(docs []json.RawMessage) {
	if len(docs) == 0 {
		return
	}
	ow.mu.Lock()
	ow.pending = append(ow.pending, docs...)
	var dropped int64
	for len(ow.pending) > maxPendingDocs {
		ow.pending = ow.pending[1:]
		dropped++
	}
	if dropped > 0 {
		ow.pendingDropped.Add(dropped)
	}
	ow.mu.Unlock()

	if dropped > 0 {
		slog.Warn("opensearch: retry queue full, dropping oldest docs", "dropped", dropped, "total_dropped", ow.pendingDropped.Load())
	}
	ow.emitStats()
}

func (ow *OpenSearchWriter) countFailedBatch(err error) {
	ow.failedBatches.Add(1)
	ow.emitStats()
	if err != nil {
		slog.Error("opensearch bulk insert failed, docs queued for retry", "error", err)
	}
}

func (ow *OpenSearchWriter) SetStatsSink(fn func(failedBatches int64, pendingDocs int, droppedDocs int64)) {
	ow.statsSink.Store(&fn)
}

func (ow *OpenSearchWriter) emitStats() {
	fnPtr := ow.statsSink.Load()
	if fnPtr == nil {
		return
	}
	ow.mu.Lock()
	p := len(ow.pending)
	ow.mu.Unlock()
	(*fnPtr)(ow.failedBatches.Load(), p, ow.pendingDropped.Load())
}

func (ow *OpenSearchWriter) PendingDocs() int {
	ow.mu.Lock()
	defer ow.mu.Unlock()
	return len(ow.pending)
}

func (ow *OpenSearchWriter) PendingDropped() int64 {
	return ow.pendingDropped.Load()
}

func (ow *OpenSearchWriter) FailedBatches() int64 {
	return ow.failedBatches.Load()
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

// doRequest returns (failedDocs, nil) on per-item errors, (nil, err) on a
// total failure, (nil, nil) on full success.
func (ow *OpenSearchWriter) doRequest(ctx context.Context, body []byte, docs []json.RawMessage) ([]json.RawMessage, error) {
	host := ow.hosts[time.Now().UnixNano()%int64(len(ow.hosts))]
	url := strings.TrimRight(host, "/") + "/_bulk"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	if ow.username != "" {
		req.SetBasicAuth(ow.username, ow.password)
	}

	resp, err := ow.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limitedBody := io.LimitReader(resp.Body, maxResponseBodyRead)

	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, limitedBody)
		return nil, fmt.Errorf("opensearch bulk insert failed: HTTP %d", resp.StatusCode)
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Status int `json:"status"`
			} `json:"index"`
		} `json:"items"`
	}
	if err := json.NewDecoder(limitedBody).Decode(&bulkResp); err != nil {
		return nil, fmt.Errorf("failed to decode bulk response: %w", err)
	}
	if !bulkResp.Errors {
		return nil, nil
	}

	var failed []json.RawMessage
	for i, item := range bulkResp.Items {
		if item.Index.Status >= 300 && i < len(docs) {
			failed = append(failed, docs[i])
		}
	}
	if len(failed) == 0 {
		return nil, nil
	}
	return failed, nil
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
