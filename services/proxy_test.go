package services

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- address parsing ---

func TestParseProxyURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string // u.String(), "" when nil
		wantErr string
	}{
		{name: "empty is no proxy", in: "", want: ""},
		{name: "spaces only", in: "   ", want: ""},
		{
			name: "bare host:port defaults to http",
			in:   "127.0.0.1:7897",
			want: "http://127.0.0.1:7897",
		},
		{
			name: "padded address is trimmed",
			in:   "  127.0.0.1:7897 ",
			want: "http://127.0.0.1:7897",
		},
		{
			name: "socks5 keeps its scheme",
			in:   "socks5://127.0.0.1:1080",
			want: "socks5://127.0.0.1:1080",
		},
		{
			name: "credentials survive",
			in:   "http://user:pa%40ss@127.0.0.1:7897",
			want: "http://user:pa%40ss@127.0.0.1:7897",
		},
		{name: "missing port", in: "127.0.0.1", wantErr: "missing port"},
		{name: "missing host", in: "http://:7897", wantErr: "missing host"},
		{name: "unknown scheme", in: "socks4://127.0.0.1:1080", wantErr: "unsupported proxy scheme"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := parseProxyURL(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("parseProxyURL(%q) = %v, want an error", tc.in, u)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %v, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseProxyURL(%q): %v", tc.in, err)
			}
			if tc.want == "" {
				if u != nil {
					t.Errorf("got %v, want nil", u)
				}
				return
			}
			if u == nil {
				t.Fatalf("got nil, want %s", tc.want)
			}
			if got := u.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- per-account resolution ---

// The four cases below are the whole rule, and the one that matters most is the
// first: an account written before this option existed carries Mode == "" and
// must follow the global setting.
func TestProxyForAccount(t *testing.T) {
	resetGlobalProxy(t)

	custom := func(mode, url string) EmailAccount {
		return EmailAccount{Email: "a@example.com", Proxy: AccountProxy{Mode: mode, URL: url}}
	}

	t.Run("follow with the global proxy off", func(t *testing.T) {
		u, err := proxyForAccount(custom(ProxyModeFollow, ""))
		if err != nil || u != nil {
			t.Fatalf("got %v, %v; want a direct connection", u, err)
		}
	})

	t.Run("follow with the global proxy on", func(t *testing.T) {
		publishGlobalProxy(ProxyConfig{Enabled: true, URL: "127.0.0.1:7897"})
		t.Cleanup(func() { resetGlobalProxy(t) })

		u, err := proxyForAccount(custom(ProxyModeFollow, ""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u == nil || u.Host != "127.0.0.1:7897" {
			t.Fatalf("got %v, want the global proxy", u)
		}
	})

	t.Run("custom wins over the global setting", func(t *testing.T) {
		publishGlobalProxy(ProxyConfig{Enabled: true, URL: "127.0.0.1:7897"})
		t.Cleanup(func() { resetGlobalProxy(t) })

		u, err := proxyForAccount(custom(ProxyModeCustom, "socks5://10.0.0.2:1080"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u == nil || u.String() != "socks5://10.0.0.2:1080" {
			t.Fatalf("got %v, want the account's own proxy", u)
		}
	})

	t.Run("off stays direct even with the global proxy on", func(t *testing.T) {
		publishGlobalProxy(ProxyConfig{Enabled: true, URL: "127.0.0.1:7897"})
		t.Cleanup(func() { resetGlobalProxy(t) })

		u, err := proxyForAccount(custom(ProxyModeOff, ""))
		if err != nil || u != nil {
			t.Fatalf("got %v, %v; want a direct connection", u, err)
		}
	})

	t.Run("custom without an address is an error, not a silent direct dial", func(t *testing.T) {
		if _, err := proxyForAccount(custom(ProxyModeCustom, "")); err == nil {
			t.Fatal("expected an error for a custom proxy with no address")
		}
	})

	t.Run("enabled global proxy with no address is an error", func(t *testing.T) {
		publishGlobalProxy(ProxyConfig{Enabled: true, URL: ""})
		t.Cleanup(func() { resetGlobalProxy(t) })

		if _, err := proxyForAccount(custom(ProxyModeFollow, "")); err == nil {
			t.Fatal("expected an error for an enabled proxy with no address")
		}
	})
}

func resetGlobalProxy(t *testing.T) {
	t.Helper()
	publishGlobalProxy(ProxyConfig{})
}

// --- HTTP CONNECT ---

// fakeConnectProxy speaks the CONNECT handshake and then writes banner as if it
// came from the mail server, which is what a real proxy does once the tunnel is
// open. It records the request line and the headers it saw.
type fakeConnectProxy struct {
	addr     string
	mu       sync.Mutex
	requests []string
	headers  []string
}

func newFakeConnectProxy(t *testing.T, reply string) *fakeConnectProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	p := &fakeConnectProxy{addr: ln.Addr().String()}

	var held []net.Conn
	var heldMu sync.Mutex
	t.Cleanup(func() {
		ln.Close()
		heldMu.Lock()
		defer heldMu.Unlock()
		for _, c := range held {
			c.Close()
		}
	})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			heldMu.Lock()
			held = append(held, conn)
			heldMu.Unlock()

			go func(c net.Conn) {
				br := bufio.NewReader(c)
				line, err := br.ReadString('\n')
				if err != nil {
					return
				}
				var hdrs []string
				for {
					h, err := br.ReadString('\n')
					if err != nil {
						return
					}
					if h == "\r\n" || h == "\n" {
						break
					}
					hdrs = append(hdrs, strings.TrimSpace(h))
				}
				p.mu.Lock()
				p.requests = append(p.requests, strings.TrimSpace(line))
				p.headers = append(p.headers, hdrs...)
				p.mu.Unlock()

				// One write: the banner must survive whatever the proxy
				// buffered, which is the case a naive implementation loses.
				_, _ = io.WriteString(c, reply)
			}(conn)
		}
	}()

	return p
}

func (p *fakeConnectProxy) url(t *testing.T) *url.URL {
	t.Helper()
	u, err := url.Parse("http://" + p.addr)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	return u
}

func (p *fakeConnectProxy) requestLines() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.requests...)
}

func TestHTTPConnectDialerTunnelsAndKeepsTheBanner(t *testing.T) {
	proxy := newFakeConnectProxy(t, "HTTP/1.1 200 Connection established\r\n\r\n* OK tunneled IMAP\r\n")

	dialer, err := newDialer(proxy.url(t), 2*time.Second)
	if err != nil {
		t.Fatalf("newDialer: %v", err)
	}
	conn, err := dialer.Dial("tcp", "imap.example.com:993")
	if err != nil {
		t.Fatalf("dial through the proxy: %v", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read the tunneled banner: %v", err)
	}
	if got := string(buf[:n]); got != "* OK tunneled IMAP\r\n" {
		t.Errorf("banner = %q, want the bytes the server sent after the 200", got)
	}

	lines := proxy.requestLines()
	if len(lines) != 1 {
		t.Fatalf("proxy saw %d requests, want 1: %v", len(lines), lines)
	}
	if lines[0] != "CONNECT imap.example.com:993 HTTP/1.1" {
		t.Errorf("request line = %q", lines[0])
	}
}

func TestHTTPConnectDialerSendsCredentials(t *testing.T) {
	proxy := newFakeConnectProxy(t, "HTTP/1.1 200 OK\r\n\r\n")

	u := proxy.url(t)
	u.User = url.UserPassword("user", "pass")
	dialer, err := newDialer(u, 2*time.Second)
	if err != nil {
		t.Fatalf("newDialer: %v", err)
	}
	conn, err := dialer.Dial("tcp", "imap.example.com:993")
	if err != nil {
		t.Fatalf("dial through the proxy: %v", err)
	}
	defer conn.Close()

	want := "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	proxy.mu.Lock()
	headers := append([]string(nil), proxy.headers...)
	proxy.mu.Unlock()

	found := false
	for _, h := range headers {
		if h == want {
			found = true
		}
	}
	if !found {
		t.Errorf("headers = %v, want one of them to be %q", headers, want)
	}
}

func TestHTTPConnectDialerReportsRefusal(t *testing.T) {
	proxy := newFakeConnectProxy(t, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")

	dialer, err := newDialer(proxy.url(t), 2*time.Second)
	if err != nil {
		t.Fatalf("newDialer: %v", err)
	}
	_, err = dialer.Dial("tcp", "imap.example.com:993")
	if err == nil {
		t.Fatal("a refused CONNECT must be an error, not a connected socket")
	}
	if !strings.Contains(err.Error(), "407") {
		t.Errorf("error = %v, want it to carry the proxy's status", err)
	}
}

// --- SOCKS5 ---

// fakeSocks5Proxy speaks the no-auth SOCKS5 handshake and then writes banner. It
// exists to prove the scheme branch really speaks SOCKS5 — and that a hostname is
// sent as a domain, so DNS happens at the proxy.
type fakeSocks5Proxy struct {
	addr string
	mu   sync.Mutex
	host string
}

func newFakeSocks5Proxy(t *testing.T, banner string) *fakeSocks5Proxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	p := &fakeSocks5Proxy{addr: ln.Addr().String()}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 512)

				// Greeting: VER, NMETHODS, METHODS…
				if _, err := io.ReadFull(c, buf[:2]); err != nil {
					return
				}
				if _, err := io.ReadFull(c, buf[:int(buf[1])]); err != nil {
					return
				}
				if _, err := c.Write([]byte{0x05, 0x00}); err != nil {
					return
				}

				// Request: VER, CMD, RSV, ATYP, ADDR, PORT
				if _, err := io.ReadFull(c, buf[:4]); err != nil {
					return
				}
				var host string
				switch buf[3] {
				case 0x01: // IPv4
					if _, err := io.ReadFull(c, buf[:4]); err != nil {
						return
					}
					host = net.IP(buf[:4]).String()
				case 0x03: // domain
					if _, err := io.ReadFull(c, buf[:1]); err != nil {
						return
					}
					n := int(buf[0])
					if _, err := io.ReadFull(c, buf[:n]); err != nil {
						return
					}
					host = string(buf[:n])
				default:
					return
				}
				if _, err := io.ReadFull(c, buf[:2]); err != nil { // port
					return
				}
				p.mu.Lock()
				p.host = host
				p.mu.Unlock()

				// Success, bound address 0.0.0.0:0.
				if _, err := c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
					return
				}
				_, _ = io.WriteString(c, banner)
			}(conn)
		}
	}()

	return p
}

func TestSocks5DialerSendsTheHostnameToTheProxy(t *testing.T) {
	proxy := newFakeSocks5Proxy(t, "* OK socks IMAP\r\n")

	u, err := url.Parse("socks5://" + proxy.addr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dialer, err := newDialer(u, 2*time.Second)
	if err != nil {
		t.Fatalf("newDialer: %v", err)
	}
	conn, err := dialer.Dial("tcp", "imap.example.com:993")
	if err != nil {
		t.Fatalf("dial through the socks proxy: %v", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "* OK socks IMAP\r\n" {
		t.Errorf("banner = %q", got)
	}

	proxy.mu.Lock()
	host := proxy.host
	proxy.mu.Unlock()
	if host != "imap.example.com" {
		t.Errorf("proxy was asked for %q, want the unresolved hostname", host)
	}
}

// --- end to end: the probe follows the account's route ---

// ProbeAccount is the meter's entry point. This pins the part that is easy to get
// wrong: it has to read the account *and* the proxy from config, then dial through
// the tunnel — not straight at the mailbox.
func TestProbeAccountUsesTheGlobalProxy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetGlobalProxy(t)

	proxy := newFakeConnectProxy(t, "HTTP/1.1 200 OK\r\n\r\n* OK proxied IMAP\r\n")

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := fmt.Sprintf(`language: en-US
proxy:
  enabled: true
  url: http://%s
email_accounts:
  - id: acc-1
    name: Test
    email: test@example.com
    imap_host: imap.example.com
    imap_port: 143
    use_tls: false
`, proxy.addr)
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	res := (&NetworkService{}).ProbeAccount("acc-1")
	if !res.Ok {
		t.Fatalf("probe failed: %+v", res)
	}

	lines := proxy.requestLines()
	if len(lines) != 1 || lines[0] != "CONNECT imap.example.com:143 HTTP/1.1" {
		t.Fatalf("proxy requests = %v, want one CONNECT to the account's server", lines)
	}
}

func TestProbeAccountWithoutAProxyConnectsDirectly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetGlobalProxy(t)

	host, port := fakeIMAPServer(t)
	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := fmt.Sprintf(`language: en-US
email_accounts:
  - id: acc-2
    email: test@example.com
    imap_host: %s
    imap_port: %d
    use_tls: false
`, host, port)
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if res := (&NetworkService{}).ProbeAccount("acc-2"); !res.Ok {
		t.Fatalf("direct probe failed: %+v", res)
	}
}

func TestProbeAccountUnknownAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	res := (&NetworkService{}).ProbeAccount("nope")
	if res.Ok || res.LatencyMS != -1 {
		t.Errorf("got %+v, want an unmeasured failure", res)
	}
	if !strings.Contains(res.Error, "account not found") {
		t.Errorf("error = %q", res.Error)
	}
}

// --- config round trip ---

// Both proxy blocks are `omitempty`: a config that never opted in must not grow a
// `proxy:` key, and one that did must come back exactly as written.
func TestProxyConfigRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetGlobalProxy(t)

	cfg := defaultConfig()
	cfg.Proxy = ProxyConfig{Enabled: true, URL: "socks5://127.0.0.1:7897"}
	cfg.EmailAccounts = []EmailAccount{
		{ID: "a", Email: "a@example.com", Proxy: AccountProxy{Mode: ProxyModeCustom, URL: "http://10.0.0.2:8080"}},
		{ID: "b", Email: "b@example.com", Proxy: AccountProxy{Mode: ProxyModeOff}},
		{ID: "c", Email: "c@example.com"},
	}

	configWriteMu.Lock()
	err := writeConfig(cfg)
	configWriteMu.Unlock()
	if err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	got, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if !got.Proxy.Enabled || got.Proxy.URL != "socks5://127.0.0.1:7897" {
		t.Errorf("global proxy = %+v", got.Proxy)
	}
	if p := got.EmailAccounts[0].Proxy; p.Mode != ProxyModeCustom || p.URL != "http://10.0.0.2:8080" {
		t.Errorf("custom account proxy = %+v", p)
	}
	if p := got.EmailAccounts[1].Proxy; p.Mode != ProxyModeOff {
		t.Errorf("off account proxy = %+v", p)
	}
	if p := got.EmailAccounts[2].Proxy; p.Mode != ProxyModeFollow {
		t.Errorf("unset account proxy = %+v, want the zero value", p)
	}

	// The cache must describe what is now on disk, without another read.
	if g := currentGlobalProxy(); !g.Enabled || g.URL != "socks5://127.0.0.1:7897" {
		t.Errorf("published global proxy = %+v, want the value just written", g)
	}
}

func TestProxyKeysAreOmittedWhenUnset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetGlobalProxy(t)

	cfg := defaultConfig()
	cfg.EmailAccounts = []EmailAccount{{ID: "a", Email: "a@example.com"}}

	configWriteMu.Lock()
	err := writeConfig(cfg)
	configWriteMu.Unlock()
	if err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "proxy:") {
		t.Errorf("a config that never set a proxy must not grow the key:\n%s", raw)
	}
}

// --- the settings page's test button ---

func TestTestProxyRejectsIncompleteAddresses(t *testing.T) {
	svc := &NetworkService{}

	if res := svc.TestProxy(""); res.Ok || res.Error != "no proxy address" {
		t.Errorf(`TestProxy("") = %+v, want "no proxy address"`, res)
	}
	res := svc.TestProxy("127.0.0.1")
	if res.Ok || !strings.Contains(res.Error, "missing port") {
		t.Errorf("TestProxy(no port) = %+v, want a missing-port error", res)
	}
}

func TestTestProxyFailureIsReportedNotThrown(t *testing.T) {
	// Bind and close to get a port nothing listens on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	res := (&NetworkService{}).TestProxy(addr)
	if res.Ok {
		t.Fatalf("an unreachable proxy must not read as working: %+v", res)
	}
	if !strings.Contains(res.Error, "unreachable") {
		t.Errorf("error = %q, want it to say the proxy is unreachable", res.Error)
	}
}
