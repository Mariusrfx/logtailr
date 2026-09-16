package ssrf

import (
	"net/http"
	"net/url"
	"testing"
)

func TestValidateExternalURL_IPLiterals(t *testing.T) {
	valid := []string{
		"http://1.1.1.1/",
		"https://8.8.8.8:443/path",
	}
	for _, u := range valid {
		if err := ValidateExternalURL(u); err != nil {
			t.Errorf("%s: expected valid, got %v", u, err)
		}
	}

	invalid := []string{
		"http://127.0.0.1/",       // loopback
		"http://127.5.6.7/",       // loopback range
		"http://0.0.0.0/",         // unspecified
		"http://[::]/",            // unspecified IPv6
		"http://[::1]/",           // loopback IPv6
		"http://10.0.0.5/",        // private
		"http://192.168.1.1/",     // private
		"http://172.16.0.1/",      // private
		"http://169.254.169.254/", // link-local (cloud metadata)
		"http://localhost/",       // well-known internal hostname
		"http://metadata.google.internal/",
		"ftp://1.1.1.1/",     // not http(s)
		"file:///etc/passwd", // not http(s)
		"http:///",           // no host
	}
	for _, u := range invalid {
		if err := ValidateExternalURL(u); err == nil {
			t.Errorf("%s: expected error, got nil", u)
		}
	}
}

func TestValidateExternalURL_UnresolvableHost(t *testing.T) {
	// The .invalid TLD is guaranteed to never resolve (RFC 6761).
	if err := ValidateExternalURL("http://nonexistent.invalid/"); err == nil {
		t.Fatal("expected error for unresolvable host")
	}
}

func TestRedirectGuard_AllowLocal(t *testing.T) {
	guard := RedirectGuard(true)
	req, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	if err := guard(req, []*http.Request{}); err != nil {
		t.Fatalf("allowLocal must skip validation, got %v", err)
	}
}

func TestRedirectGuard_BlocksInternalTarget(t *testing.T) {
	guard := RedirectGuard(false)
	orig, _ := url.Parse("https://example.com/")
	origReq, _ := http.NewRequest(http.MethodGet, orig.String(), nil)

	target, _ := url.Parse("http://169.254.169.254/latest/meta-data/")
	req, _ := http.NewRequest(http.MethodGet, target.String(), nil)
	if err := guard(req, []*http.Request{origReq}); err == nil {
		t.Fatal("redirect to internal IP must be rejected")
	}

	// Public IP literal target is allowed.
	target, _ = url.Parse("https://1.1.1.1/")
	req, _ = http.NewRequest(http.MethodGet, target.String(), nil)
	if err := guard(req, []*http.Request{origReq}); err != nil {
		t.Fatalf("public target should be allowed, got %v", err)
	}
}

func TestRedirectGuard_MaxRedirects(t *testing.T) {
	guard := RedirectGuard(false)
	orig, _ := url.Parse("https://example.com/")
	origReq, _ := http.NewRequest(http.MethodGet, orig.String(), nil)

	target, _ := url.Parse("https://1.1.1.1/")
	req, _ := http.NewRequest(http.MethodGet, target.String(), nil)

	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = origReq
	}
	if err := guard(req, via); err == nil {
		t.Fatal("redirect chain over 10 hops must be rejected")
	}
}
