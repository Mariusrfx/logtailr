package alert

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// EmailConfig holds SMTP configuration for the email notifier.
type EmailConfig struct {
	Host     string
	Port     int
	From     string
	To       []string
	Username string
	Password string
	TLS      bool
}

// EmailNotifier sends alert events via SMTP email.
type EmailNotifier struct {
	cfg EmailConfig
}

// NewEmailNotifier creates an email notifier with the given SMTP configuration.
func NewEmailNotifier(cfg EmailConfig) (*EmailNotifier, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("email: host is required")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("email: from address is required")
	}
	if len(cfg.To) == 0 {
		return nil, fmt.Errorf("email: at least one recipient is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return &EmailNotifier{cfg: cfg}, nil
}

func (n *EmailNotifier) Notify(event *Event) error {
	subject := fmt.Sprintf("[Logtailr][%s] %s", strings.ToUpper(event.Severity), event.Rule)
	body := fmt.Sprintf("Rule: %s\nSeverity: %s\nSource: %s\nTime: %s\n\n%s",
		event.Rule, event.Severity, event.Source, event.Timestamp.Format("2006-01-02 15:04:05 MST"), event.Message)

	if event.Count > 0 {
		body += fmt.Sprintf("\nCount: %d", event.Count)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		n.cfg.From, strings.Join(n.cfg.To, ","), subject, body)

	addr := fmt.Sprintf("%s:%d", n.cfg.Host, n.cfg.Port)

	var auth smtp.Auth
	if n.cfg.Username != "" {
		auth = smtp.PlainAuth("", n.cfg.Username, n.cfg.Password, n.cfg.Host)
	}

	if n.cfg.TLS {
		return n.sendTLS(addr, auth, msg)
	}
	return smtp.SendMail(addr, auth, n.cfg.From, n.cfg.To, []byte(msg))
}

func (n *EmailNotifier) sendTLS(addr string, auth smtp.Auth, msg string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: n.cfg.Host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("email: TLS dial failed: %w", err)
	}

	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("email: SMTP client failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("email: auth failed: %w", err)
		}
	}

	if err := c.Mail(n.cfg.From); err != nil {
		return fmt.Errorf("email: MAIL FROM failed: %w", err)
	}
	for _, to := range n.cfg.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("email: RCPT TO %q failed: %w", to, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: DATA failed: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("email: write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close failed: %w", err)
	}

	return c.Quit()
}

func (n *EmailNotifier) Close() error { return nil }
