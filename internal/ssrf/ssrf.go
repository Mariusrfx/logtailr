// Package ssrf provides server-side request forgery prevention helpers
// shared by config validation and outbound HTTP clients.
package ssrf

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DNSResolveTimeout bounds DNS lookups performed during URL validation.
const DNSResolveTimeout = 3 * time.Second

// IsDisallowedIP reports whether an IP must never be reachable by external
// URL validation (SSRF prevention).
func IsDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// ValidateExternalURL checks that a URL is http(s) and does not target
// internal networks (SSRF prevention). IP literals are checked directly;
// hostnames are resolved and every IP they point to must be public.
func ValidateExternalURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("must start with http:// or https://")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Check for well-known dangerous hostnames
	if host == "localhost" || host == "metadata.google.internal" {
		return fmt.Errorf("internal hostname %q not allowed", host)
	}

	// Check if it's an IP literal pointing to internal networks
	if ip := net.ParseIP(host); ip != nil {
		if IsDisallowedIP(ip) {
			return fmt.Errorf("internal/private IP address %q not allowed", host)
		}
		return nil
	}

	// Hostname: resolve and make sure none of the addresses are internal.
	ctx, cancel := context.WithTimeout(context.Background(), DNSResolveTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("host %q cannot be resolved: %v", host, err)
	}
	for _, a := range addrs {
		if IsDisallowedIP(a.IP) {
			return fmt.Errorf("host %q resolves to internal/private IP %q", host, a.IP)
		}
	}

	return nil
}

// RedirectGuard returns an http.Client CheckRedirect function that re-validates
// every redirect target, so an allowed URL cannot redirect the client to an
// internal address. When allowLocal is true, no validation is performed.
func RedirectGuard(allowLocal bool) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if allowLocal {
			return nil
		}
		return ValidateExternalURL(req.URL.String())
	}
}
