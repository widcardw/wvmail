package services

// OAuth2 online verification for Gmail and Microsoft accounts (Settings →
// Accounts → 在线验证). The flow is the desktop-standard Authorization Code +
// PKCE + loopback redirect (RFC 8252): the app opens the system browser, the
// provider redirects back to a one-shot HTTP server on the loopback interface,
// and the code is exchanged for tokens on the spot.
//
// Tokens: the refresh token is returned to the frontend and stored on the
// account (encrypted like every other secret); short-lived access tokens are
// exchanged on demand by oauthAccessToken and cached in memory only.
//
// Both providers authenticate IMAP with the same SASL XOAUTH2 mechanism, so
// the mail layers do not care which provider issued the token.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// oauthProviderConfig carries one provider's endpoints. TokenURL/UserinfoURL
// are fields rather than constants so tests can point them at httptest servers.
type oauthProviderConfig struct {
	AuthURL       string
	TokenURL      string
	UserinfoURL   string
	Scopes        []string
	RefreshScopes []string
	// AuthParams are appended to the authorization URL (Google wants
	// access_type=offline for refresh tokens; Microsoft benefits from
	// prompt=select_account when several Microsoft accounts share a browser).
	AuthParams map[string]string
	// DefaultClientID is a public client ID embedded with the app build. Users
	// can still override it in the online-verification form when needed.
	DefaultClientID string
}

var oauthProviders = map[string]oauthProviderConfig{
	"google": {
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
		// userinfo email needs the email scope alongside the mail scope.
		Scopes:      []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
		UserinfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
		AuthParams:  map[string]string{"access_type": "offline", "prompt": "consent"},
	},
	"microsoft": {
		AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		// The authorization request can collect consent for multiple APIs. The
		// code exchange and refresh requests below then ask for one resource at
		// a time: Graph for profile lookup, Outlook for IMAP/SMTP.
		Scopes: []string{
			"offline_access",
			"openid",
			"email",
			"https://graph.microsoft.com/User.Read",
			"https://outlook.office.com/IMAP.AccessAsUser.All",
			"https://outlook.office.com/SMTP.Send",
		},
		RefreshScopes: []string{
			"https://outlook.office.com/IMAP.AccessAsUser.All",
			"https://outlook.office.com/SMTP.Send",
		},
		DefaultClientID: "63884bde-31f3-4d9b-854d-097342c1d1c1",
		UserinfoURL:     "https://graph.microsoft.com/v1.0/me",
		AuthParams:      map[string]string{"prompt": "select_account"},
	},
}

// oauthSessionTimeout bounds how long the user has to finish the browser
// round trip before the session is torn down and Poll reports a timeout.
const oauthSessionTimeout = 5 * time.Minute

// OAuthService runs the online-verification flow. One service instance is
// shared by the whole app; the frontend drives it with BeginOAuth + PollOAuth.
type OAuthService struct {
	mu       sync.Mutex
	sessions map[string]*oauthSession
	client   *http.Client
}

func NewOAuthService() *OAuthService {
	return &OAuthService{
		sessions: make(map[string]*oauthSession),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

type oauthSession struct {
	provider    string
	cfg         oauthProviderConfig
	clientID    string
	verifier    string
	redirectURI string
	srv         *http.Server

	done   chan struct{}
	mu     sync.Mutex
	result *oauthAuthResult
	err    error
}

type oauthAuthResult struct {
	Email        string
	Provider     string
	RefreshToken string
}

// OAuthPollResult is what the frontend sees from PollOAuth. A session stays
// NotDone until the browser round trip finished (or failed); a finished
// session is consumed by the first poll that reports it.
type OAuthPollResult struct {
	Done         bool   `json:"done"`
	Email        string `json:"email,omitempty"`
	Provider     string `json:"provider,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Error        string `json:"error,omitempty"`
}

// listenOAuthCallback creates a loopback listener and its corresponding
// redirect URI. Microsoft ignores the port only for localhost redirect URIs,
// so bind both IPv4 and IPv6 loopback where available and use localhost there.
// Google continues to use the IPv4 literal accepted by its loopback flow.
func listenOAuthCallback(provider string) ([]net.Listener, string, error) {
	ln4, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	port := ln4.Addr().(*net.TCPAddr).Port
	listeners := []net.Listener{ln4}
	host := "127.0.0.1"
	if provider == "microsoft" {
		host = "localhost"
		ln6, err := net.Listen("tcp6", net.JoinHostPort("::1", fmt.Sprint(port)))
		if err == nil {
			listeners = append(listeners, ln6)
		}
	}
	return listeners, fmt.Sprintf("http://%s:%d/callback", host, port), nil
}

func closeOAuthListeners(listeners []net.Listener) {
	for _, ln := range listeners {
		_ = ln.Close()
	}
}

// BeginOAuth starts one authorization session: it opens a loopback listener,
// builds the provider's authorization URL, and returns it. The frontend opens
// the URL in the system browser (OsService.OpenURL) and then polls PollOAuth
// with the state that is embedded in the returned URL.
func (s *OAuthService) BeginOAuth(provider, clientID string) (string, error) {
	cfg, ok := oauthProviders[provider]
	if !ok {
		return "", fmt.Errorf("unsupported OAuth provider %q (supported: google, microsoft)", provider)
	}
	if strings.TrimSpace(clientID) == "" {
		clientID = cfg.DefaultClientID
	}
	if strings.TrimSpace(clientID) == "" {
		return "", fmt.Errorf(
			"no OAuth client ID configured for %s. Register a \"Mobile and desktop applications\" app in the Azure portal "+
				"(https://aka.ms/appregistrations, supported account types: personal + org, "+
				"redirect URI http://localhost/callback) and paste its client ID into the form",
			provider)
	}

	listeners, redirectURI, err := listenOAuthCallback(provider)
	if err != nil {
		return "", fmt.Errorf("loopback listener: %w", err)
	}

	state, err := oauthRandomHex(16)
	if err != nil {
		closeOAuthListeners(listeners)
		return "", err
	}
	verifier, err := oauthRandomB64(48)
	if err != nil {
		closeOAuthListeners(listeners)
		return "", err
	}
	challenge := sha256.Sum256([]byte(verifier))
	challengeB64 := base64.RawURLEncoding.EncodeToString(challenge[:])

	ses := &oauthSession{
		provider:    provider,
		cfg:         cfg,
		clientID:    clientID,
		verifier:    verifier,
		redirectURI: redirectURI,
		done:        make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.callbackHandler(state, ses))
	ses.srv = &http.Server{Handler: mux}
	for _, ln := range listeners {
		ln := ln
		go func() {
			if serveErr := ses.srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				slog.Warn("oauth: loopback server stopped", "error", serveErr)
				s.finishSession(state, ses, nil, fmt.Errorf("callback server stopped: %w", serveErr))
			}
		}()
	}
	// Bound the whole round trip: without this, an abandoned session keeps a
	// port open and Poll would report "pending" forever.
	time.AfterFunc(oauthSessionTimeout, func() {
		s.finishSession(state, ses, nil, fmt.Errorf("authorization timed out after %s", oauthSessionTimeout))
	})

	s.mu.Lock()
	s.sessions[state] = ses
	s.mu.Unlock()

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(cfg.Scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", challengeB64)
	q.Set("code_challenge_method", "S256")
	for k, v := range cfg.AuthParams {
		q.Set(k, v)
	}
	return cfg.AuthURL + "?" + q.Encode(), nil
}

// PollOAuth reports the state of one session. The first poll that sees a
// finished session consumes it (the session is removed afterwards), so the
// frontend can stop polling on Done — success or failure alike.
func (s *OAuthService) PollOAuth(state string) (OAuthPollResult, error) {
	if state == "" {
		return OAuthPollResult{}, fmt.Errorf("empty OAuth state")
	}
	s.mu.Lock()
	ses := s.sessions[state]
	s.mu.Unlock()
	if ses == nil {
		return OAuthPollResult{}, fmt.Errorf("unknown or already consumed OAuth session")
	}

	select {
	case <-ses.done:
		s.mu.Lock()
		delete(s.sessions, state)
		s.mu.Unlock()
		ses.mu.Lock()
		result, err := ses.result, ses.err
		ses.mu.Unlock()
		if err != nil {
			return OAuthPollResult{Done: true, Provider: ses.provider, Error: err.Error()}, nil
		}
		return OAuthPollResult{
			Done:         true,
			Email:        result.Email,
			Provider:     result.Provider,
			RefreshToken: result.RefreshToken,
		}, nil
	default:
		return OAuthPollResult{Done: false}, nil
	}
}

// CancelOAuth abandons a session the user gave up on, releasing the port.
func (s *OAuthService) CancelOAuth(state string) error {
	s.mu.Lock()
	ses := s.sessions[state]
	delete(s.sessions, state)
	s.mu.Unlock()
	if ses == nil {
		return fmt.Errorf("unknown OAuth session")
	}
	s.finishSession(state, ses, nil, errors.New("authorization cancelled"))
	return nil
}

// callbackHandler consumes the provider's redirect: it validates the state and
// hands the code (or the error) to the session, then shows a short page so the
// browser tab is not left hanging on a blank screen.
func (s *OAuthService) callbackHandler(state string, ses *oauthSession) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			desc := q.Get("error_description")
			s.finishSession(state, ses, nil, fmt.Errorf("authorization denied: %s (%s)", e, desc))
			s.renderCallbackPage(w, false)
			return
		}
		if q.Get("state") != state {
			// Not our session (or a replay): leave it for the timeout to reap.
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			s.finishSession(state, ses, nil, errors.New("authorization response had no code"))
			s.renderCallbackPage(w, false)
			return
		}
		// Exchange synchronously: the redirect tab then shows the real outcome,
		// and the first Poll after the browser returns deterministically sees
		// the finished session (no exchange/ 页面 race).
		res, err := s.exchangeCode(ses, code)
		s.finishSession(state, ses, res, err)
		s.renderCallbackPage(w, err == nil)
	}
}

func (s *OAuthService) renderCallbackPage(w http.ResponseWriter, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	title, body := "授权成功", "正在获取账户信息，请返回 wvmail。"
	if !ok {
		title, body = "授权未完成", "出现错误，请返回 wvmail 查看详情后重试。"
	}
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>%s - wvmail</title>
<body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh">
<div style="text-align:center"><h2>%s</h2><p>%s</p></div></body>`, title, title, body)
}

// finishSession records the outcome once and tears the listener down. Later
// calls (timeout after success, double callback) are no-ops.
func (s *OAuthService) finishSession(state string, ses *oauthSession, result *oauthAuthResult, err error) {
	ses.mu.Lock()
	alreadyDone := false
	select {
	case <-ses.done:
		alreadyDone = true
	default:
	}
	if !alreadyDone {
		ses.result, ses.err = result, err
		close(ses.done)
	}
	ses.mu.Unlock()
	if alreadyDone {
		return
	}
	if result == nil && err != nil {
		slog.Warn("oauth: authorization session finished with error", "provider", ses.provider, "error", err)
	} else if result != nil {
		slog.Info("oauth: authorization finished", "provider", ses.provider, "email", result.Email)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = ses.srv.Shutdown(shutdownCtx)
}

// exchangeCode swaps the authorization code for tokens and then reads the
// account email from the provider's profile endpoint.
func (s *OAuthService) exchangeCode(ses *oauthSession, code string) (*oauthAuthResult, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {ses.redirectURI},
		"client_id":     {ses.clientID},
		"code_verifier": {ses.verifier},
	}
	if ses.provider == "microsoft" {
		// The authorization request asks consent for both Graph and Outlook.
		// An access token is audience-bound, so redeem this code for the Graph
		// profile scope only; Outlook gets its own token during IMAP refresh.
		form.Set("scope", "https://graph.microsoft.com/User.Read")
	}
	tokens, err := s.postForm(ses.cfg.TokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	if tokens.RefreshToken == "" {
		return nil, errors.New("provider returned no refresh token (offline access missing)")
	}
	email, err := s.fetchEmail(ses.cfg, tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	return &oauthAuthResult{Email: email, Provider: ses.provider, RefreshToken: tokens.RefreshToken}, nil
}

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *OAuthService) postForm(tokenURL string, form url.Values) (*oauthTokens, error) {
	resp, err := s.client.PostForm(tokenURL, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		oauthTokens
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("malformed token response (HTTP %d): %s", resp.StatusCode, truncateForLog(body))
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("%s: %s", parsed.Error, parsed.ErrorDescription)
	}
	if resp.StatusCode != http.StatusOK || parsed.AccessToken == "" {
		return nil, fmt.Errorf("unexpected token response (HTTP %d): %s", resp.StatusCode, truncateForLog(body))
	}
	return &parsed.oauthTokens, nil
}

func (s *OAuthService) fetchEmail(cfg oauthProviderConfig, accessToken string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.UserinfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("profile request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("profile request failed (HTTP %d): %s", resp.StatusCode, truncateForLog(body))
	}
	return parseOAuthEmail(cfg, body)
}

// parseOAuthEmail picks the account's real mailbox address out of a profile
// payload. Microsoft note (Pebble issue #78): a Microsoft account can log in
// with an external address (e.g. a QQ mail), where `email`/`userPrincipalName`
// is that login name while `mail` is the actual Outlook mailbox — so for
// Microsoft, `mail` wins.
func parseOAuthEmail(cfg oauthProviderConfig, body []byte) (string, error) {
	var profile struct {
		Email             string `json:"email"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		return "", fmt.Errorf("malformed profile response: %w", err)
	}
	email := strings.TrimSpace(profile.Email)
	if strings.Contains(cfg.UserinfoURL, "graph.microsoft.com") {
		for _, candidate := range []string{profile.Mail, profile.UserPrincipalName, profile.Email} {
			if c := strings.TrimSpace(candidate); c != "" {
				email = c
				break
			}
		}
	}
	if email == "" {
		return "", errors.New("profile response contains no email address")
	}
	return email, nil
}

func truncateForLog(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func oauthRandomHex(bytesN int) (string, error) {
	b := make([]byte, bytesN)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func oauthRandomB64(bytesN int) (string, error) {
	b := make([]byte, bytesN)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// --- access tokens for IMAP ---

type oauthCachedToken struct {
	token  string
	expiry time.Time
}

// oauthTokenCache holds short-lived access tokens per account. Only the access
// token lives here; the refresh token is persisted on the account. If a
// provider rotates the refresh token during a refresh (Microsoft may), the new
// value is logged and dropped — rotation is not forced for these clients, and
// persisting it would need a config write on a background path.
var oauthTokenCache = struct {
	sync.Mutex
	entries map[string]oauthCachedToken
}{entries: make(map[string]oauthCachedToken)}

// oauthAccessToken returns a valid access token for the account's IMAP login,
// refreshing it when expired or close to expiry.
func oauthAccessToken(account EmailAccount) (string, error) {
	if account.AuthType != "oauth2" {
		return "", fmt.Errorf("account %s does not use OAuth", account.ID)
	}
	cfg, ok := oauthProviders[account.OAuthProvider]
	if !ok {
		return "", fmt.Errorf("unknown OAuth provider %q", account.OAuthProvider)
	}
	clientID := strings.TrimSpace(account.OAuthClientID)
	if clientID == "" {
		clientID = cfg.DefaultClientID
	}
	if clientID == "" {
		return "", fmt.Errorf("account has no OAuth client ID")
	}
	if account.OAuthRefreshToken == "" {
		return "", fmt.Errorf("account has no refresh token; re-run online verification")
	}

	oauthTokenCache.Lock()
	if cached, ok := oauthTokenCache.entries[account.ID]; ok && time.Now().Before(cached.expiry) {
		oauthTokenCache.Unlock()
		return cached.token, nil
	}
	oauthTokenCache.Unlock()

	svc := &OAuthService{client: &http.Client{Timeout: 30 * time.Second}}
	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {account.OAuthRefreshToken},
		"client_id":     {clientID},
	}
	if len(cfg.RefreshScopes) > 0 {
		refreshForm.Set("scope", strings.Join(cfg.RefreshScopes, " "))
	}
	tokens, err := svc.postForm(cfg.TokenURL, refreshForm)
	if err != nil {
		return "", fmt.Errorf("refresh failed: %w", err)
	}
	if tokens.AccessToken == "" {
		return "", errors.New("refresh response had no access token")
	}
	if tokens.RefreshToken != "" && tokens.RefreshToken != account.OAuthRefreshToken {
		slog.Warn("oauth: provider rotated the refresh token; keeping the stored one",
			"account", account, "provider", account.OAuthProvider)
	}
	expiry := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	if tokens.ExpiresIn <= 0 {
		expiry = time.Now().Add(30 * time.Minute)
	}
	oauthTokenCache.Lock()
	oauthTokenCache.entries[account.ID] = oauthCachedToken{token: tokens.AccessToken, expiry: expiry.Add(-time.Minute)}
	oauthTokenCache.Unlock()
	return tokens.AccessToken, nil
}

// xoauth2Client implements sasl.Client for Google/Microsoft's XOAUTH2
// mechanism: one initial response "user=<u>\x01auth=Bearer <t>\x01\x01" and no
// further challenges. XOAUTH2 uses SOH (0x01) delimiters, not NUL bytes.
type xoauth2Client struct {
	username string
	token    string
}

func (c xoauth2Client) Start() (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + c.username + "\x01auth=Bearer " + c.token + "\x01\x01"), nil
}

func (c xoauth2Client) Next(challenge []byte) ([]byte, error) {
	// A challenge here is the server rejecting the token (base64 JSON error);
	// responding again cannot help, so surface it instead of looping.
	return nil, fmt.Errorf("imap: XOAUTH2 rejected: %s", string(challenge))
}
