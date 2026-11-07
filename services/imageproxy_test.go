package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func imageProxyRequest(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/image-proxy?url="+url.QueryEscape(target), nil)
}

func TestImageProxyFetchesImageAndStripsIdentifiers(t *testing.T) {
	var hits atomic.Int32
	var sawCookie, sawReferer, sawUserAgent string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sawCookie = r.Header.Get("Cookie")
		sawReferer = r.Header.Get("Referer")
		sawUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=60")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer upstream.Close()

	p := newImageProxy(true)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/logo.png"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "png-bytes" {
		t.Errorf("body = %q, want png-bytes", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=60" {
		t.Errorf("cache control = %q, want private, max-age=60", got)
	}
	if sawCookie != "" || sawReferer != "" {
		t.Errorf("upstream saw cookie=%q referer=%q, want neither", sawCookie, sawReferer)
	}
	if !strings.Contains(sawUserAgent, "wvmail") {
		t.Errorf("user agent = %q, want a wvmail marker", sawUserAgent)
	}

	// The second request must come from the cache, not the sender's server.
	rec = httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/logo.png"))
	if rec.Code != http.StatusOK {
		t.Fatalf("cached status = %d, want 200", rec.Code)
	}
	if hits.Load() != 1 {
		t.Errorf("upstream hits = %d, want 1 (cached)", hits.Load())
	}
}

func TestImageProxyCacheTTLHonoursNoCache(t *testing.T) {
	cases := map[string]time.Duration{
		"no-store":            0,
		"no-cache":            0,
		"private, no-cache":   0,
		"max-age=60":          60 * time.Second,
		"public, max-age=120": 120 * time.Second,
		"max-age=999999999":   imageProxyMaxMaxAge,
		"":                    imageProxyDefaultMaxAge,
	}
	for value, want := range cases {
		header := http.Header{}
		if value != "" {
			header.Set("Cache-Control", value)
		}
		if got := imageProxyCacheTTL(header); got != want {
			t.Errorf("imageProxyCacheTTL(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestImageProxyRejectsNonImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer upstream.Close()

	p := newImageProxy(true)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/page.html"))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestImageProxyAllowsFontsForCss(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write([]byte("font-bytes"))
	}))
	defer upstream.Close()

	p := newImageProxy(true)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/mail.woff2"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS header = %q, want *", got)
	}
}

func TestImageProxyRejectsSvgWithPossibleExternalReferences(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg><image href="http://tracker.example/p.png"/></svg>`))
	}))
	defer upstream.Close()

	p := newImageProxy(true)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/icon.svg"))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestImageProxyUsesExtensionWhenContentTypeIsGeneric(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("not-really-a-png"))
	}))
	defer upstream.Close()

	p := newImageProxy(true)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/logo.png"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
}

func TestImageProxyRejectsOversizedBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("1234567890"))
	}))
	defer upstream.Close()

	p := newImageProxy(true)
	p.maxBytes = 4
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(upstream.URL+"/big.png"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestImageProxyRejectsPrivateTargets(t *testing.T) {
	p := NewImageProxy()
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest("http://127.0.0.1:8080/logo.png"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("loopback status = %d, want 502", rec.Code)
	}

	rec = httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest("http://169.254.169.254/latest/meta-data/"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("metadata status = %d, want 502", rec.Code)
	}
}

func TestImageProxyRejectsInvalidTargets(t *testing.T) {
	p := NewImageProxy()
	cases := []string{
		"file:///etc/passwd",
		"http://user:" + "pw" + "@example.com/a.png",
		"http:///missing-host.png",
		"ftp://example.com/a.png",
		"http://example.com:22/a.png",
	}
	for _, target := range cases {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, imageProxyRequest(target))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestImageProxyDropsRefererOnRedirect(t *testing.T) {
	var sawReferer string
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawReferer = r.Header.Get("Referer")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/final.png", http.StatusFound)
	}))
	defer redirect.Close()

	p := newImageProxy(true)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, imageProxyRequest(redirect.URL+"/start.png"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if sawReferer != "" {
		t.Errorf("redirect target saw referer %q, want none", sawReferer)
	}
}

func TestImageProxyRejectsOtherMethods(t *testing.T) {
	p := NewImageProxy()
	req := httptest.NewRequest(http.MethodPost, "/image-proxy?url=http%3A%2F%2Fexample.com%2Fa.png", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestIsPublicIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":         true,
		"1.1.1.1":         true,
		"2606:4700::1111": true,
		"127.0.0.1":       false,
		"10.0.0.1":        false,
		"172.16.0.1":      false,
		"192.168.1.1":     false,
		"169.254.169.254": false,
		"100.64.0.1":      false,
		"0.0.0.0":         false,
		"224.0.0.1":       false,
		"::1":             false,
		"fc00::1":         false,
		"fe80::1":         false,
	}
	for raw, want := range cases {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if got := isPublicIP(addr); got != want {
			t.Errorf("isPublicIP(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestImageProxyRefusesPrivateHostBeforeUsingAProxy(t *testing.T) {
	p := NewImageProxy()
	for _, host := range []string{"127.0.0.1", "localhost", "169.254.169.254"} {
		if err := p.validateProxyHost(context.Background(), host); err == nil {
			t.Errorf("validateProxyHost(%q) = nil, want an error", host)
		}
	}
}

func TestIsFakeIP(t *testing.T) {
	cases := map[string]bool{
		"198.18.0.1":   true,
		"198.19.255.1": true,
		"8.8.8.8":      false,
		"192.168.1.1":  false,
	}
	for raw, want := range cases {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if got := isFakeIP(addr); got != want {
			t.Errorf("isFakeIP(%s) = %v, want %v", raw, got, want)
		}
	}
}
