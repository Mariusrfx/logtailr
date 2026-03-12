package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const webhookHTTPTimeout = 10 * time.Second

type Notifier interface {
	Notify(event *Event) error
	Close() error
}

type ConsoleNotifier struct{}

func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{}
}

func (n *ConsoleNotifier) Notify(event *Event) error {
	prefix := "[ALERT]"
	if event.Severity == string(SeverityCritical) {
		prefix = "\033[31m[ALERT][CRITICAL]\033[0m"
	} else {
		prefix = "\033[33m[ALERT][WARNING]\033[0m"
	}

	msg := fmt.Sprintf("%s %s: %s", prefix, event.Rule, event.Message)
	if event.Source != "" {
		msg = fmt.Sprintf("%s (source: %s)", msg, event.Source)
	}

	_, err := fmt.Fprintln(os.Stderr, msg)
	return err
}

func (n *ConsoleNotifier) Close() error { return nil }

type WebhookNotifier struct {
	client *http.Client
	url    string
}

func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		client: &http.Client{Timeout: webhookHTTPTimeout},
		url:    url,
	}
}

func (n *WebhookNotifier) Notify(event *Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("alert webhook: failed to marshal event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), webhookHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("alert webhook: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("alert webhook: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("alert webhook: HTTP %d", resp.StatusCode)
	}

	return nil
}

func (n *WebhookNotifier) Close() error { return nil }
