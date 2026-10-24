package services

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Talking to the server: the connection (TCP + TLS + login, with the provider
// quirks that 163/126/QQ need), the user-facing error wrapping, and the
// connection test behind Settings → Accounts.

// providerNeedsIDCommand checks whether an account's IMAP server expects an
// RFC 2971 ID command before LOGIN. NetEase's Coremail servers (163.com,
// 126.com) and Tencent's QQMail servers (qq.com, plus the exmail.qq.com
// endpoint used by many universities' enterprise mail) reject unidentifying
// clients with "Unsafe Login" or leave the session hanging.
//
// The provider label is checked first, but an account configured as "custom"
// still needs identification when it points at one of those servers — e.g. a
// university mailbox hosted on imap.exmail.qq.com. That is why the host is
// consulted as well.
func providerNeedsIDCommand(account EmailAccount) bool {
	switch account.Provider {
	case "163", "126", "qq":
		return true
	}
	host := strings.ToLower(strings.TrimSpace(account.IMAPHost))
	for _, suffix := range []string{"qq.com", "163.com", "126.com"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// TestConnection attempts to connect and authenticate to the IMAP server.
// Returns success status and a human-readable message. Times out after 15 seconds.
func (s *EmailService) TestConnection(account EmailAccount) (bool, string) {
	// The account list and the edit form both end up here with (in principle)
	// the same account object, so log the exact inputs and timing: if the two
	// buttons ever disagree, the log shows which server/user/password state each
	// one actually used instead of leaving it to guesswork.
	slog.Info("imap: testing connection",
		"account", account,
		"host", account.IMAPHost,
		"port", account.IMAPPort,
		"user", account.IMAPUser,
		"provider", account.Provider,
		"auth_type", account.AuthType,
		"use_tls", account.UseTLS,
		"has_password", account.IMAPPass != "")

	start := time.Now()
	client, err := dialIMAP(account, 15*time.Second)
	if err != nil {
		slog.Error("imap: connection test failed",
			"account", account, "host", account.IMAPHost,
			"elapsed", time.Since(start).Round(time.Millisecond), "error", err)
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer client.Logout()

	slog.Info("imap: connection test succeeded",
		"account", account, "host", account.IMAPHost,
		"elapsed", time.Since(start).Round(time.Millisecond))
	return true, fmt.Sprintf("Successfully connected to %s as %s", account.IMAPHost, account.IMAPUser)
}

// dialIMAP connects to the IMAP server with the given timeout for the entire
// connection + login operation. The timeout covers TCP dial, TLS handshake,
// and IMAP login.
func dialIMAP(account EmailAccount, timeout time.Duration) (*imapclient.Client, error) {
	return dialIMAPWith(account, timeout, nil)
}

// dialIMAPWith is dialIMAP plus the unsolicited-response handler the IDLE
// watcher needs. It has to be installed at dial time: the client routes
// untagged EXISTS/EXPUNGE responses to it, and there is no way to attach one to
// an already-open connection.
func dialIMAPWith(account EmailAccount, timeout time.Duration, handler *imapclient.UnilateralDataHandler) (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", account.IMAPHost, account.IMAPPort)

	// The proxy is resolved per dial, not cached on the account: flipping it in
	// Settings has to affect the next connection, and the account object the
	// frontend holds is not re-read on every operation.
	dialer, err := dialerForAccount(account, timeout)
	if err != nil {
		slog.Error("imap connect failed (proxy)",
			"host", addr, "user", account.IMAPUser,
			"phase", "proxy", "error", err)
		return nil, fmt.Errorf("proxy: %w", err)
	}

	opts := &imapclient.Options{
		TLSConfig: &tls.Config{
			ServerName: account.IMAPHost,
		},
		UnilateralDataHandler: handler,
	}

	// Each phase is timed and logged: a stall that ends at ~timeout during
	// "connect" means the TCP/TLS handshake never completed, which is a
	// different fault from a rejected credential or a failed command.
	connectStart := time.Now()

	// The socket is opened here rather than by imapclient.DialTLS because
	// Options.Dialer is a concrete *net.Dialer and cannot be handed a proxy
	// dialer. What follows is what DialTLS / DialStartTLS / DialInsecure do
	// internally — dial, wrap in TLS where the account uses it, then New.
	var client *imapclient.Client
	var dialErr error

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		dialErr = err
	} else {
		// Bound the handshake with the caller's timeout. The tls.DialWithDialer
		// path this replaces bounded only the TCP connect, so a server (or a
		// proxy) that accepted the socket and then stalled could hold the call
		// well past it.
		if timeout > 0 {
			_ = conn.SetDeadline(time.Now().Add(timeout))
		}
		if account.UseTLS || account.IMAPPort == 993 {
			tlsConn := tls.Client(conn, opts.TLSConfig)
			if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
				_ = conn.Close()
				dialErr = handshakeErr
			} else {
				client = imapclient.New(tlsConn, opts)
			}
		} else {
			client, dialErr = imapclient.NewStartTLS(conn, opts)
			if dialErr != nil {
				// Fallback to insecure plain connection
				slog.Warn("imap STARTTLS failed, retrying insecurely",
					"host", addr, "user", account.IMAPUser, "error", dialErr)
				plain, plainErr := dialer.Dial("tcp", addr)
				if plainErr != nil {
					dialErr = plainErr
				} else {
					client = imapclient.New(plain, opts)
					dialErr = nil
				}
			}
		}
		if client != nil {
			// The handshake is over; the client sets its own per-command
			// deadlines from here on.
			_ = conn.SetDeadline(time.Time{})
		}
	}
	if dialErr != nil {
		slog.Error("imap connect failed (tcp+tls)",
			"host", addr, "user", account.IMAPUser,
			"phase", "connect",
			"elapsed", time.Since(connectStart).Round(time.Millisecond),
			"timeout", timeout, "error", dialErr)
		return nil, fmt.Errorf("dial failed: %w", dialErr)
	}
	connectElapsed := time.Since(connectStart)

	// Some Coremail/Tencent providers (163.com, qq.com, exmail.qq.com) require an
	// RFC 2971 ID command before LOGIN. Without it they reject with "Unsafe
	// Login" or leave the connection hanging.
	if providerNeedsIDCommand(account) {
		// Wait for the server greeting first (consumes the greeting from the
		// connection so the client is ready for the next command).
		if err := client.WaitGreeting(); err != nil {
			client.Close()
			slog.Error("imap greeting failed",
				"host", addr, "user", account.IMAPUser,
				"phase", "greeting",
				"elapsed", time.Since(connectStart).Round(time.Millisecond),
				"error", err)
			return nil, fmt.Errorf("greeting failed: %w", err)
		}
		// Send the ID command with client identification.
		idCmd := client.ID(&imap.IDData{
			Name:    "wvmail",
			Version: "1.0",
			Vendor:  "wvmail",
		})
		if _, err := idCmd.Wait(); err != nil {
			// Not fatal: some servers answer BAD/NO to ID yet still accept
			// LOGIN. Log it and let the LOGIN step report the real problem,
			// instead of failing with an opaque "id command failed".
			slog.Warn("imap ID command rejected, continuing with LOGIN",
				"host", addr, "user", account.IMAPUser, "error", err)
		}
	}

	// Login with timeout — use a goroutine + select so login itself can't hang
	// beyond the configured timeout. OAuth2 accounts authenticate with SASL
	// XOAUTH2 instead of LOGIN: the password field is empty, and the refresh
	// token is exchanged (and cached) right here on the connection path.
	type loginResult struct {
		err error
	}
	loginStart := time.Now()
	done := make(chan loginResult, 1)
	go func() {
		if account.AuthType == "oauth2" {
			done <- loginResult{err: oauthIMAPLogin(client, account)}
			return
		}
		cmd := client.Login(account.IMAPUser, account.IMAPPass)
		err := cmd.Wait()
		done <- loginResult{err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			client.Close()
			slog.Error("imap login failed",
				"host", addr, "user", account.IMAPUser,
				"phase", "login",
				"connect_elapsed", connectElapsed.Round(time.Millisecond),
				"elapsed", time.Since(loginStart).Round(time.Millisecond),
				"error", res.err)
			return nil, wrapIMAPError("login failed", res.err)
		}
		// Send CAPABILITY after login — some servers (e.g., 163) require this
		// handshake before they'll allow SELECT/EXAMINE.
		if capCmd := client.Capability(); capCmd != nil {
			_, capErr := capCmd.Wait()
			if capErr != nil {
				client.Close()
				slog.Error("imap capability failed",
					"host", addr, "user", account.IMAPUser, "error", capErr)
				return nil, fmt.Errorf("capability failed: %w", capErr)
			}
		}
		slog.Debug("imap connected",
			"host", addr, "user", account.IMAPUser,
			"connect_elapsed", connectElapsed.Round(time.Millisecond),
			"login_elapsed", time.Since(loginStart).Round(time.Millisecond))
		return client, nil
	case <-time.After(timeout):
		client.Close()
		slog.Error("imap login timed out",
			"host", addr, "user", account.IMAPUser,
			"phase", "login",
			"connect_elapsed", connectElapsed.Round(time.Millisecond),
			"timeout", timeout)
		return nil, fmt.Errorf("login timed out after %v", timeout)
	}
}

// oauthIMAPLogin authenticates an already-connected IMAP client with SASL
// XOAUTH2, fetching (and caching) a fresh access token first. Microsoft requires
// the mechanism's SOH (0x01) delimiters; xoauth2Client builds that payload.
func oauthIMAPLogin(client *imapclient.Client, account EmailAccount) error {
	token, err := oauthAccessToken(account)
	if err != nil {
		return fmt.Errorf("oauth token: %w", err)
	}
	return client.Authenticate(xoauth2Client{username: account.IMAPUser, token: token})
}

// wrapIMAPError wraps common IMAP errors with user-friendly context.
func wrapIMAPError(context string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "Unsafe Login") {
		return fmt.Errorf("%s: %w\n\nThis server rejected the connection as unsafe. This may happen with 163/126 mail even when using the correct authorization code. Try: (1) ensure IMAP/SMTP service is enabled in webmail settings, (2) generate a fresh authorization code, (3) contacting customer service (kefu@188.com).", context, err)
	}
	if strings.Contains(msg, "authentication failed") || strings.Contains(msg, "login failed") || strings.Contains(msg, "LOGIN failed") || strings.Contains(msg, "Authentication failed") {
		return fmt.Errorf("%s: %w\n\nHint: Check your username/password. Some providers (163, QQ) require an authorization code (授权码) instead of your login password.", context, err)
	}
	return fmt.Errorf("%s: %w", context, err)
}
