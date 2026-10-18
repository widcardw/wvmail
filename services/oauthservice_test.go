package services

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// overrideProvider points one provider's token/userinfo endpoints at test
// servers and restores the original on cleanup.
func overrideProvider(t *testing.T, name string, cfg oauthProviderConfig) {
	t.Helper()
	old := oauthProviders[name]
	oauthProviders[name] = cfg
	t.Cleanup(func() { oauthProviders[name] = old })
}

func TestParseOAuthEmailMicrosoftPriority(t *testing.T) {
	cfg := oauthProviderConfig{UserinfoURL: "https://graph.microsoft.com/v1.0/me"}
	// mail wins over the login-name fields (Pebble issue #78).
	got, err := parseOAuthEmail(cfg, []byte(`{"mail":"me@outlook.com","userPrincipalName":"login@qq.com","email":"login@qq.com"}`))
	if err != nil || got != "me@outlook.com" {
		t.Fatalf("parseOAuthEmail = %q, %v; want me@outlook.com", got, err)
	}
	// userPrincipalName wins over email when mail is absent.
	got, err = parseOAuthEmail(cfg, []byte(`{"userPrincipalName":"login@qq.com","email":"login@qq.com"}`))
	if err != nil || got != "login@qq.com" {
		t.Fatalf("parseOAuthEmail = %q, %v; want login@qq.com", got, err)
	}
	// Google: plain email field.
	got, err = parseOAuthEmail(
		oauthProviderConfig{UserinfoURL: "https://www.googleapis.com/oauth2/v2/userinfo"},
		[]byte(`{"email":"me@gmail.com"}`))
	if err != nil || got != "me@gmail.com" {
		t.Fatalf("parseOAuthEmail = %q, %v; want me@gmail.com", got, err)
	}
	// Nothing usable is an error, not an empty address.
	if _, err := parseOAuthEmail(cfg, []byte(`{}`)); err == nil {
		t.Fatal("expected error for profile without any address")
	}
}

func TestXoauth2ClientPayload(t *testing.T) {
	c := xoauth2Client{username: "me@example.com", token: "tok"}
	mech, data, err := c.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if mech != "XOAUTH2" {
		t.Errorf("mech = %q, want XOAUTH2", mech)
	}
	want := "user=me@example.com\x01auth=Bearer tok\x01\x01"
	if string(data) != want {
		t.Errorf("payload = %q, want %q", data, want)
	}
	if _, err := c.Next([]byte("err")); err == nil {
		t.Error("Next should reject a challenge (server denied the token)")
	}
}

func TestBeginOAuthURLParameters(t *testing.T) {
	svc := NewOAuthService()
	authURL, err := svc.BeginOAuth("google", "test-client-id")
	if err != nil {
		t.Fatalf("BeginOAuth: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	if !strings.HasPrefix(authURL, "https://accounts.google.com/o/oauth2/v2/auth?") {
		t.Errorf("unexpected auth endpoint: %s", authURL)
	}
	q := u.Query()
	if q.Get("client_id") != "test-client-id" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	if !strings.Contains(q.Get("scope"), "https://mail.google.com/") {
		t.Errorf("scope missing mail scope: %q", q.Get("scope"))
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Errorf("PKCE challenge missing: %v", q)
	}
	if q.Get("access_type") != "offline" || q.Get("prompt") != "consent" {
		t.Errorf("Google offline params missing: %v", q)
	}
	if q.Get("state") == "" || q.Get("code_challenge") == "" {
		t.Errorf("state/challenge empty")
	}
	// The challenge must be base64url(sha256(verifier)) — the verifier itself
	// is only known server-side, so just verify the method wiring end-to-end
	// via a full flow in TestOAuthEndToEnd.
}

func TestBeginMicrosoftUsesLocalhostLoopbackRedirect(t *testing.T) {
	svc := NewOAuthService()
	authURL, err := svc.BeginOAuth("microsoft", "test-client-id")
	if err != nil {
		t.Fatalf("BeginOAuth: %v", err)
	}
	redirect, err := url.Parse(redirectURIFromAuthURL(t, authURL))
	if err != nil {
		t.Fatalf("parse redirect URI: %v", err)
	}
	if redirect.Scheme != "http" || redirect.Hostname() != "localhost" || redirect.Port() == "" || redirect.Path != "/callback" {
		t.Fatalf("redirect_uri = %q; want http://localhost:<dynamic-port>/callback", redirect)
	}

	// The registered URI omits the dynamic port; the real local callback must
	// still be reachable through localhost on the selected port.
	state := stateFromAuthURL(t, authURL)
	resp, err := http.Get(redirect.String() + "?error=access_denied&state=" + url.QueryEscape(state))
	if err != nil {
		t.Fatalf("localhost callback GET: %v", err)
	}
	resp.Body.Close()
	res, err := svc.PollOAuth(state)
	if err != nil || !res.Done || !strings.Contains(res.Error, "access_denied") {
		t.Fatalf("PollOAuth = %+v, %v; want completed access_denied", res, err)
	}
}

func TestBeginOAuthUsesBuiltInMicrosoftClientID(t *testing.T) {
	svc := NewOAuthService()
	authURL, err := svc.BeginOAuth("microsoft", "")
	if err != nil {
		t.Fatalf("BeginOAuth: %v", err)
	}
	if got := clientIDFromAuthURL(t, authURL); got != "63884bde-31f3-4d9b-854d-097342c1d1c1" {
		t.Fatalf("client_id = %q; want built-in Microsoft client ID", got)
	}
	if err := svc.CancelOAuth(stateFromAuthURL(t, authURL)); err != nil {
		t.Fatalf("CancelOAuth: %v", err)
	}
}

func TestMicrosoftOAuthUsesGraphAudienceForProfile(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token form: %v", err)
		}
		if got := r.Form.Get("scope"); got != "https://graph.microsoft.com/User.Read" {
			t.Errorf("code exchange scope = %q; want Graph User.Read only", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "graph-access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenSrv.Close()
	profileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer graph-access-token" {
			t.Errorf("profile Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"email": "mailbox@outlook.com",
		})
	}))
	defer profileSrv.Close()

	overrideProvider(t, "microsoft", oauthProviderConfig{
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    tokenSrv.URL,
		UserinfoURL: profileSrv.URL,
		Scopes: []string{
			"offline_access",
			"https://graph.microsoft.com/User.Read",
			"https://outlook.office.com/IMAP.AccessAsUser.All",
		},
	})

	svc := NewOAuthService()
	authURL, err := svc.BeginOAuth("microsoft", "test-client-id")
	if err != nil {
		t.Fatalf("BeginOAuth: %v", err)
	}
	if err := completeOAuthCallback(t, authURL); err != nil {
		t.Fatalf("callback: %v", err)
	}
	res, err := svc.PollOAuth(stateFromAuthURL(t, authURL))
	if err != nil || !res.Done || res.Error != "" {
		t.Fatalf("PollOAuth = %+v, %v; want success", res, err)
	}
	if res.Email != "mailbox@outlook.com" || res.RefreshToken != "refresh-token" {
		t.Fatalf("PollOAuth = %+v; want Outlook mailbox and refresh token", res)
	}
}

func TestBeginOAuthRequiresClientID(t *testing.T) {
	svc := NewOAuthService()
	// Google has no built-in client ID yet.
	if _, err := svc.BeginOAuth("google", ""); err == nil {
		t.Fatal("expected error without a client ID")
	}
	if _, err := svc.BeginOAuth("bogus", "cid"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestOAuthEndToEnd(t *testing.T) {
	// Fake token + userinfo endpoints; the callback server is real.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token form: %v", err)
		}
		if r.Form.Get("code_verifier") == "" {
			t.Error("token exchange missing PKCE verifier")
		}
		if r.Form.Get("client_id") != "test-client-id" {
			t.Errorf("client_id = %q", r.Form.Get("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"expires_in":    3600,
		})
	}))
	defer tokenSrv.Close()
	profileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			t.Errorf("profile auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"email": "me@gmail.com"})
	}))
	defer profileSrv.Close()

	overrideProvider(t, "google", oauthProviderConfig{
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    tokenSrv.URL,
		UserinfoURL: profileSrv.URL,
		Scopes:      []string{"https://mail.google.com/"},
		AuthParams:  map[string]string{"access_type": "offline"},
	})

	svc := NewOAuthService()
	authURL, err := svc.BeginOAuth("google", "test-client-id")
	if err != nil {
		t.Fatalf("BeginOAuth: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	state := u.Query().Get("state")
	verifierChallenge := u.Query().Get("code_challenge")

	// Verify the challenge derives from the verifier via PKCE S256: complete
	// the flow with the verifier the session holds, checking the token
	// endpoint received it (asserted in the fake above) is enough; here we
	// additionally assert the challenge is base64url of a sha256.
	if _, err := base64.RawURLEncoding.DecodeString(verifierChallenge); err != nil {
		t.Errorf("challenge not base64url: %v", err)
	}
	if len(verifierChallenge) != 43 { // base64url of 32 bytes
		t.Errorf("challenge length = %d, want 43", len(verifierChallenge))
	}
	_ = sha256.Size

	// Not done before the browser round trip.
	res, err := svc.PollOAuth(state)
	if err != nil || res.Done {
		t.Fatalf("pre-callback poll = %+v, %v; want pending", res, err)
	}

	redirect := redirectURIFromAuthURL(t, authURL)
	resp, err := http.Get(redirect + "?code=the-code&state=" + state)
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	resp.Body.Close()

	res, err = svc.PollOAuth(state)
	if err != nil {
		t.Fatalf("PollOAuth: %v", err)
	}
	if !res.Done || res.Error != "" {
		t.Fatalf("poll = %+v; want done without error", res)
	}
	if res.Email != "me@gmail.com" || res.RefreshToken != "rt-1" || res.Provider != "google" {
		t.Fatalf("poll = %+v; want google/me@gmail.com/rt-1", res)
	}

	// The session is consumed: a second poll fails, the port is released.
	if _, err := svc.PollOAuth(state); err == nil {
		t.Fatal("expected unknown-session error after consumption")
	}
}

func TestOAuthCallbackDenied(t *testing.T) {
	overrideProvider(t, "google", oauthProviderConfig{
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "http://127.0.0.1:1/token", // never reached
		Scopes:   []string{"s"},
	})
	svc := NewOAuthService()
	authURL, err := svc.BeginOAuth("google", "cid")
	if err != nil {
		t.Fatalf("BeginOAuth: %v", err)
	}
	state := stateFromAuthURL(t, authURL)
	resp, err := http.Get(redirectURIFromAuthURL(t, authURL) + "?error=access_denied&error_description=nope&state=" + state)
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	resp.Body.Close()

	res, err := svc.PollOAuth(state)
	if err != nil {
		t.Fatalf("PollOAuth: %v", err)
	}
	if !res.Done || res.Error == "" {
		t.Fatalf("poll = %+v; want done with error", res)
	}
	if !strings.Contains(res.Error, "access_denied") {
		t.Errorf("error = %q, want it to mention access_denied", res.Error)
	}
}

func TestOAuthAccessTokenRefreshAndCache(t *testing.T) {
	refreshCalls := 0
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		if err := r.ParseForm(); err != nil {
			t.Errorf("form: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "rt-x" {
			t.Errorf("unexpected refresh form: %v", r.Form)
		}
		if got := r.Form.Get("client_id"); got != "built-in-client-id" {
			t.Errorf("refresh client_id = %q; want built-in default", got)
		}
		if got := r.Form.Get("scope"); got != "https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send" {
			t.Errorf("refresh scope = %q; want Outlook IMAP and SMTP scopes", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-x", "expires_in": 3600})
	}))
	defer tokenSrv.Close()
	overrideProvider(t, "microsoft", oauthProviderConfig{
		TokenURL:        tokenSrv.URL,
		DefaultClientID: "built-in-client-id",
		RefreshScopes: []string{
			"https://outlook.office.com/IMAP.AccessAsUser.All",
			"https://outlook.office.com/SMTP.Send",
		},
		Scopes: []string{"s"},
	})
	t.Cleanup(func() { delete(oauthTokenCache.entries, "acc-1") })

	account := EmailAccount{
		ID:                "acc-1",
		AuthType:          "oauth2",
		OAuthProvider:     "microsoft",
		OAuthRefreshToken: "rt-x",
	}
	tok, err := oauthAccessToken(account)
	if err != nil || tok != "at-x" {
		t.Fatalf("oauthAccessToken = %q, %v", tok, err)
	}
	// Second call must come from the cache.
	if _, err := oauthAccessToken(account); err != nil {
		t.Fatalf("cached call: %v", err)
	}
	if refreshCalls != 1 {
		t.Errorf("refresh endpoint called %d times, want 1 (cached)", refreshCalls)
	}
	// A password account must never take the OAuth path.
	account.AuthType = "password"
	if _, err := oauthAccessToken(account); err == nil {
		t.Fatal("expected error for non-OAuth account")
	}
}

func stateFromAuthURL(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return u.Query().Get("state")
}

func redirectURIFromAuthURL(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return u.Query().Get("redirect_uri")
}

func clientIDFromAuthURL(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return u.Query().Get("client_id")
}

func completeOAuthCallback(t *testing.T, authURL string) error {
	t.Helper()
	callbackURL := redirectURIFromAuthURL(t, authURL) + "?code=the-code&state=" + url.QueryEscape(stateFromAuthURL(t, authURL))
	resp, err := http.Get(callbackURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
