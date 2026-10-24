package services

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

// One optional proxy, shared by everything the app dials: IMAP (per account,
// with an override), LLM/translation/OAuth (always the global one).
//
// The shape is deliberately small — one address, one on/off — because that is
// what a user with a local proxy tool actually has ("my tool listens on
// 127.0.0.1:7897"). There is no PAC, no per-request rules, no environment
// variable support: HTTP_PROXY in the shell is ignored on purpose, so what the
// settings page shows is exactly what the app does.

// ProxyConfig is the app-wide proxy (Settings → Proxy).
type ProxyConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
	// URL is "host:port", optionally with a scheme ("socks5://…") and
	// credentials ("http://user:pass@host:port"). A bare address is treated as
	// an HTTP proxy — see defaultProxyScheme.
	URL string `yaml:"url,omitempty"`
}

// AccountProxy is one account's override of the global proxy.
type AccountProxy struct {
	// Mode is "" (follow the global proxy), ProxyModeCustom or ProxyModeOff.
	// The empty string is the zero value, i.e. what an account written before
	// this option existed gets — following the global setting.
	Mode string `yaml:"mode,omitempty"`
	// URL is only read when Mode is ProxyModeCustom.
	URL string `yaml:"url,omitempty"`
}

const (
	// ProxyModeFollow is the zero value: use the global proxy if it is on.
	ProxyModeFollow = ""
	// ProxyModeCustom routes this account through its own address, whether or
	// not the global proxy is enabled.
	ProxyModeCustom = "custom"
	// ProxyModeOff keeps this account on a direct connection even when the
	// global proxy is on — for a mailbox on the local network that must not be
	// sent through a remote tunnel.
	ProxyModeOff = "off"
)

// defaultProxyScheme is what a bare "host:port" means. Every proxy tool exposes
// an HTTP port, and the "mixed" ports of Clash/v2ray answer both HTTP CONNECT
// and SOCKS5 — so HTTP is the guess that works most often. A SOCKS-only
// endpoint has to say so with a scheme.
const defaultProxyScheme = "http"

// connectHandshakeTimeout bounds the CONNECT exchange when the caller's context
// carries no deadline and the forward dialer has no timeout of its own.
const connectHandshakeTimeout = 30 * time.Second

// globalProxy caches the AppConfig.Proxy that was read last, so dial-time code
// (which has no ConfigService, and must not re-read YAML on every folder open)
// can resolve it. readConfig and writeConfig both publish here: every read
// refreshes it, so it cannot go stale after a settings change.
var globalProxy atomic.Pointer[ProxyConfig]

// publishGlobalProxy stores the current global proxy settings.
func publishGlobalProxy(cfg ProxyConfig) {
	p := cfg
	globalProxy.Store(&p)
}

// currentGlobalProxy returns the last published global proxy settings.
func currentGlobalProxy() ProxyConfig {
	if p := globalProxy.Load(); p != nil {
		return *p
	}
	return ProxyConfig{}
}

// parseProxyURL normalises a user-typed proxy address. A nil URL means "no
// proxy", which is also what an empty string returns.
func parseProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !strings.Contains(raw, "://") {
		raw = defaultProxyScheme + "://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %w", raw, err)
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (use http:// or socks5://)", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("invalid proxy address %q: missing host", raw)
	}
	// No default port: guessing 8080 for a tool listening on 7897 produces a
	// connection refused that looks like "the proxy is broken" instead of "the
	// address is incomplete".
	if u.Port() == "" {
		return nil, fmt.Errorf("invalid proxy address %q: missing port", raw)
	}
	return u, nil
}

// globalProxyURL resolves the global proxy, or nil when it is off.
func globalProxyURL() (*url.URL, error) {
	cfg := currentGlobalProxy()
	if !cfg.Enabled {
		return nil, nil
	}
	u, err := parseProxyURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("the proxy is enabled but has no address")
	}
	return u, nil
}

// proxyForAccount resolves the proxy one account's IMAP connection should use.
// A nil URL means a direct connection.
func proxyForAccount(account EmailAccount) (*url.URL, error) {
	switch account.Proxy.Mode {
	case ProxyModeOff:
		return nil, nil
	case ProxyModeCustom:
		u, err := parseProxyURL(account.Proxy.URL)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, fmt.Errorf("account %s uses a custom proxy but has no address", account.Email)
		}
		return u, nil
	}
	return globalProxyURL()
}

// connDialer is the part of net.Dialer this package needs: Dial for the IMAP
// client and the probe, DialContext for http.Transport.
type connDialer interface {
	Dial(network, addr string) (net.Conn, error)
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// dialerForAccount returns the dialer an account's IMAP connection must use.
func dialerForAccount(account EmailAccount, timeout time.Duration) (connDialer, error) {
	u, err := proxyForAccount(account)
	if err != nil {
		return nil, err
	}
	return newDialer(u, timeout)
}

// newDialer builds a dialer for u, or a plain TCP dialer when u is nil.
func newDialer(u *url.URL, timeout time.Duration) (connDialer, error) {
	base := &net.Dialer{Timeout: timeout}
	if u == nil {
		return base, nil
	}

	switch u.Scheme {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if u.User != nil {
			auth = &proxy.Auth{User: u.User.Username()}
			if pw, ok := u.User.Password(); ok {
				auth.Password = pw
			}
		}
		// x/net/proxy sends a hostname as a SOCKS5 domain, so DNS is resolved by
		// the proxy — the same behaviour as socks5h, which is what a user in a
		// censored network needs (local DNS for imap.gmail.com does not resolve).
		d, err := proxy.SOCKS5("tcp", u.Host, auth, base)
		if err != nil {
			return nil, fmt.Errorf("socks5 proxy %s: %w", u.Host, err)
		}
		return socksDialer{d: d}, nil
	case "http", "https":
		return &httpConnectDialer{
			proxyAddr:  u.Host,
			authHeader: proxyAuthHeader(u),
			forward:    base,
		}, nil
	}
	return nil, fmt.Errorf("unsupported proxy scheme %q", u.Scheme)
}

// proxyAuthHeader builds the Proxy-Authorization value for credentials embedded
// in the address ("http://user:pass@host:port"), or "" when there are none.
func proxyAuthHeader(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	pw, _ := u.User.Password()
	token := base64.StdEncoding.EncodeToString([]byte(u.User.Username() + ":" + pw))
	return "Basic " + token
}

// socksDialer adapts x/net/proxy's Dialer to connDialer. The static type
// returned by proxy.SOCKS5 only promises Dial, but the concrete dialer also
// implements DialContext, which is what http.Transport uses to honour request
// cancellation.
type socksDialer struct {
	d proxy.Dialer
}

func (s socksDialer) Dial(network, addr string) (net.Conn, error) {
	return s.d.Dial(network, addr)
}

func (s socksDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if cd, ok := s.d.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, addr)
	}
	return s.d.Dial(network, addr)
}

// httpConnectDialer tunnels through an HTTP proxy with CONNECT, which is the one
// mechanism every HTTP proxy supports and the only one that can carry IMAP.
type httpConnectDialer struct {
	proxyAddr  string
	authHeader string
	forward    *net.Dialer
}

func (d *httpConnectDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *httpConnectDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("proxy: unsupported network %q", network)
	}

	conn, err := d.forward.DialContext(ctx, "tcp", d.proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("proxy %s unreachable: %w", d.proxyAddr, err)
	}

	// Bound the CONNECT exchange itself: a proxy that accepts the socket and
	// then never answers would otherwise hold the caller for its whole timeout
	// with nothing to show for it.
	deadline, ok := ctx.Deadline()
	if !ok {
		if d.forward.Timeout > 0 {
			deadline = time.Now().Add(d.forward.Timeout)
		} else {
			deadline = time.Now().Add(connectHandshakeTimeout)
		}
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("proxy %s: %w", d.proxyAddr, err)
	}

	br, err := d.handshake(conn, addr)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	// The tunnel is open: hand the deadline back to the IMAP client, which sets
	// its own per-command ones.
	_ = conn.SetDeadline(time.Time{})
	return &bufferedConn{Conn: conn, r: br}, nil
}

// handshake writes the CONNECT request and reads the response headers. It
// returns the reader holding whatever the proxy (or the mail server, whose
// banner arrives as soon as the tunnel opens) sent past the blank line — those
// bytes are the first thing the IMAP client reads, so they must not be lost.
func (d *httpConnectDialer) handshake(conn net.Conn, addr string) (*bufio.Reader, error) {
	var req strings.Builder
	fmt.Fprintf(&req, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if d.authHeader != "" {
		fmt.Fprintf(&req, "Proxy-Authorization: %s\r\n", d.authHeader)
	}
	req.WriteString("\r\n")
	if _, err := io.WriteString(conn, req.String()); err != nil {
		return nil, fmt.Errorf("proxy %s: CONNECT failed: %w", d.proxyAddr, err)
	}

	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("proxy %s: no CONNECT response: %w", d.proxyAddr, err)
	}
	if !strings.Contains(statusLine, " 200") {
		return nil, fmt.Errorf("proxy %s refused CONNECT to %s: %s",
			d.proxyAddr, addr, strings.TrimSpace(statusLine))
	}
	// Drain the remaining headers up to the blank line. Anything after it is
	// tunnel payload and stays in br.
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("proxy %s: truncated CONNECT response: %w", d.proxyAddr, err)
		}
		if line == "\r\n" || line == "\n" {
			return br, nil
		}
	}
}

// bufferedConn returns the bytes buffered during the CONNECT handshake before
// reading from the socket.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

// newHTTPClient builds a client for outbound HTTP (LLM, translation, OAuth) that
// follows the global proxy.
//
// A broken proxy setting is reported by every request rather than swallowed: the
// transport's dialer returns the parse error, so the user sees "proxy: …" in the
// LLM test dialog instead of a request that quietly went out another way.
func newHTTPClient(timeout time.Duration) *http.Client {
	u, err := globalProxyURL()
	if err != nil {
		return &http.Client{Timeout: timeout, Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return nil, fmt.Errorf("proxy: %w", err)
			},
		}}
	}

	d, derr := newDialer(u, timeout)
	if derr != nil {
		return &http.Client{Timeout: timeout, Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return nil, fmt.Errorf("proxy: %w", derr)
			},
		}}
	}

	if u != nil {
		// Debug, and never the raw address: it can carry credentials.
		slog.Debug("proxy: routing http through proxy", "proxy", u.Redacted())
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:       d.DialContext,
			ForceAttemptHTTP2: true,
			// HTTP_PROXY / HTTPS_PROXY in the environment are ignored on purpose:
			// the settings page is the single source of truth.
			Proxy: nil,
		},
	}
}
