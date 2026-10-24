package services

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

// NetworkService answers three questions for the connection UI: can the mail
// server be reached *right now*, and how long does it take to answer (the
// title-bar meter); which accounts currently hold a live IMAP IDLE connection
// (the sidebar's per-account "disconnected" marker); and does a proxy address
// actually work (Settings → Proxy).
//
// It exists because the webview cannot time an IMAP connection by itself, and
// navigator.onLine only reports whether some interface is up — not whether the
// mailbox answers. The frontend polls this while the window is visible and turns
// the latency into the meter's four states (see
// frontend/src/composables/connection.ts).
//
// The probe deliberately stops before LOGIN, so it says nothing about whether an
// account's credentials still work — that is the second question's job, and it is
// answered from the IDLE connections that already exist instead of by dialling.
type NetworkService struct {
	// idle is the IDLE watcher registry, injected before the window opens and
	// never reassigned. Nil reads as "no account is idling", which is what tests
	// and an unwired service see.
	idle IdleStateReporter
}

// IdleStateReporter is the part of EmailIdleService this service needs. An
// interface rather than the concrete type so the two stay independent: the IDLE
// watcher has no idea it is being reported on, and this service does not need a
// whole mail service to answer one question.
type IdleStateReporter interface {
	IdleStates() map[string]bool
}

// NewNetworkService wires the source of the per-account connection states.
//
// A constructor rather than a setter: an exported Set*(interface) method would be
// exported to the frontend as a binding too, and the binding generator cannot
// serialise an interface parameter (it warns and emits a method that can only be
// called with null).
func NewNetworkService(idle IdleStateReporter) *NetworkService {
	return &NetworkService{idle: idle}
}

// AccountIdleStates reports, for every account with a live IDLE watcher, whether
// that watcher is connected right now. Accounts without a watcher (auto-sync off,
// or a server that refused IDLE) are absent rather than false — the caller has to
// be able to tell "not watched" from "watcher is in its reconnect backoff".
func (s *NetworkService) AccountIdleStates() map[string]bool {
	if s.idle == nil {
		return map[string]bool{}
	}
	return s.idle.IdleStates()
}

// NetworkProbeResult is one measurement. LatencyMS is -1 when the probe failed,
// so "0 ms" can never be confused with "no answer"; the frontend keys "Off" off
// the failure, not off a magic latency.
type NetworkProbeResult struct {
	Ok        bool
	LatencyMS int
	// Attempts is 1 when the server answered first try, 2 when the first
	// handshake failed and the retry succeeded — the difference between
	// "slow" and "flaky", which the tooltip surfaces.
	Attempts int
	Error    string
}

const (
	// probeTimeout bounds one attempt end to end: TCP connect, TLS handshake,
	// and the server's IMAP banner. It is far below the 60s a real mail
	// operation gets — past a few seconds the meter's job is to say "bad", not
	// to wait for the mail to arrive.
	probeTimeout = 3 * time.Second
	// probeAttempts: one failed handshake is not proof of a bad network (Wi-Fi
	// roaming, a DNS hiccup, a server dropping the first connection), so the
	// probe retries once before it reports the connection as down.
	probeAttempts = 2
	// probeRetryDelay is short: the retry is there to filter one-off failures,
	// not to space out load.
	probeRetryDelay = 250 * time.Millisecond
	// maxProbeErrorLen keeps a server's full error text from ending up in a
	// tooltip line.
	maxProbeErrorLen = 160

	// proxyTestHost/Port is what Settings → Proxy tests a proxy against. A
	// Google IMAP endpoint is the case a proxy exists for in the first place,
	// and reaching it proves the tunnel carries IMAP + TLS — not merely that
	// some TCP port somewhere answered.
	proxyTestHost = "imap.gmail.com"
	proxyTestPort = 993
	// proxyTestTimeout is longer than probeTimeout: the request is one the user
	// pressed a button for, and a proxy hop adds a round trip to every phase.
	proxyTestTimeout = 8 * time.Second
)

// ProbeAccount measures the mail server of one account, through whatever route
// that account uses: direct, its own proxy, or the global one. That is why the
// input is the account ID rather than host/port — resolving the route is the same
// decision `dialIMAP` makes, and it must not be made twice, in two languages.
func (s *NetworkService) ProbeAccount(accountID string) NetworkProbeResult {
	cfg, err := readConfig()
	if err != nil {
		return NetworkProbeResult{LatencyMS: -1, Error: truncateText(err.Error(), maxProbeErrorLen)}
	}
	var account *EmailAccount
	for i := range cfg.EmailAccounts {
		if cfg.EmailAccounts[i].ID == accountID {
			account = &cfg.EmailAccounts[i]
			break
		}
	}
	if account == nil {
		return NetworkProbeResult{LatencyMS: -1, Error: "account not found"}
	}

	dialer, err := dialerForAccount(*account, probeTimeout)
	if err != nil {
		return NetworkProbeResult{LatencyMS: -1, Error: truncateText(err.Error(), maxProbeErrorLen)}
	}
	return probeMailbox(account.IMAPHost, account.IMAPPort, account.UseTLS, dialer, probeTimeout)
}

// TestProxy answers "does this address actually work?" for Settings → Proxy. The
// address comes from the form rather than from the config, so one can be tried
// before it is saved.
func (s *NetworkService) TestProxy(proxyURL string) NetworkProbeResult {
	u, err := parseProxyURL(proxyURL)
	if err != nil {
		return NetworkProbeResult{LatencyMS: -1, Error: truncateText(err.Error(), maxProbeErrorLen)}
	}
	if u == nil {
		return NetworkProbeResult{LatencyMS: -1, Error: "no proxy address"}
	}
	dialer, err := newDialer(u, proxyTestTimeout)
	if err != nil {
		return NetworkProbeResult{LatencyMS: -1, Error: truncateText(err.Error(), maxProbeErrorLen)}
	}

	// Debug, and never the raw address: it can carry credentials.
	slog.Debug("network: testing proxy", "proxy", u.Redacted())
	return probeMailbox(proxyTestHost, proxyTestPort, true, dialer, proxyTestTimeout)
}

// probeMailbox measures host:port over an already-resolved route. A failure is
// reported in the result rather than as a Go error: "unreachable" is a normal
// outcome the UI renders, not an exceptional one.
func probeMailbox(host string, port int, useTLS bool, dialer connDialer, timeout time.Duration) NetworkProbeResult {
	host = strings.TrimSpace(host)
	res := NetworkProbeResult{Ok: false, LatencyMS: -1}

	if host == "" || port <= 0 {
		res.Error = "no IMAP host configured"
		return res
	}

	var lastErr error
	for attempt := 1; attempt <= probeAttempts; attempt++ {
		res.Attempts = attempt

		elapsed, err := probeIMAPReachable(host, port, useTLS, dialer, timeout)
		if err == nil {
			res.Ok = true
			res.LatencyMS = int(elapsed.Milliseconds())
			slog.Debug("network: probe ok",
				"host", host, "port", port, "attempt", attempt, "latency_ms", res.LatencyMS)
			return res
		}
		lastErr = err

		if attempt < probeAttempts {
			time.Sleep(probeRetryDelay)
		}
	}

	res.Error = truncateText(strings.TrimSpace(lastErr.Error()), maxProbeErrorLen)
	// Debug, not Error: an offline machine would otherwise write an error line
	// every few seconds for as long as the window stays open, burying the errors
	// the sync path logs once per real failure.
	slog.Debug("network: probe failed",
		"host", host, "port", port, "attempts", res.Attempts, "error", res.Error)
	return res
}

// probeIMAPReachable connects through dialer, completes TLS where the account
// uses it, and reads the IMAP banner, returning how long the server took to
// answer.
//
// This deliberately does *not* go through dialIMAP/imapclient: the probe runs
// every few seconds and must never outlive its own timeout, while go-imap applies
// its own 30s response timeout inside DialStartTLS/WaitGreeting — measured here,
// a silent server made a "3 second" probe take 60. One SetDeadline over a raw
// socket bounds the whole exchange instead.
//
// It also stops before LOGIN: a login per tick would open a session and spend a
// credential attempt every few seconds, and a rate limit or a rejected password
// is not a network problem (the sync path reports those on its own).
//
// The plaintext branch checks TCP + banner and skips STARTTLS on purpose: dialIMAP
// falls back to a plaintext connection when STARTTLS is refused, so a plaintext
// account that answers its banner is one the mail path can use.
func probeIMAPReachable(host string, port int, useTLS bool, dialer connDialer, timeout time.Duration) (time.Duration, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	start := time.Now()

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return time.Since(start), fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close()

	// One deadline covers every read and write below, including the TLS handshake.
	if err := conn.SetDeadline(deadline); err != nil {
		return time.Since(start), fmt.Errorf("set deadline: %w", err)
	}

	stream := net.Conn(conn)
	if useTLS || port == 993 {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: host,
			NextProtos: []string{"imap"},
		})
		if err := tlsConn.Handshake(); err != nil {
			return time.Since(start), fmt.Errorf("tls handshake failed: %w", err)
		}
		stream = tlsConn
	}

	banner, err := readBanner(stream)
	latency := time.Since(start)
	if err != nil {
		return latency, fmt.Errorf("no banner: %w", err)
	}
	// "* OK"/"* PREAUTH"/"* BYE" is what an IMAP server greets with. Anything
	// else is a captive portal, a proxy, or a plain HTTP port — reachable, but
	// not usable, so it must not read as a healthy connection.
	if !strings.HasPrefix(banner, "*") {
		return latency, fmt.Errorf("not an IMAP server: %s", truncateText(banner, 60))
	}

	return latency, nil
}

// readBanner reads whatever the server sends first. It stops at the first bytes
// that arrive instead of waiting for a full line: the banner starts with "*", and
// requiring the newline would fold a client-side round trip into a measurement of
// the server.
func readBanner(conn net.Conn) (string, error) {
	buf := make([]byte, 512)
	for i := 0; i < 4; i++ {
		n, err := conn.Read(buf)
		if n > 0 {
			return strings.TrimSpace(string(buf[:n])), nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("no data")
}

// truncateText shortens s to at most max runes, marking that it was cut. Runes,
// not bytes, so a CJK error message is not sliced mid-character.
func truncateText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
