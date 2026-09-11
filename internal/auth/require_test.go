//nolint:testpackage // covers unexported helpers
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	return newTestProviderWith(t, config.AuthConfig{}, nil)
}

//nolint:paralleltest // t.Setenv via newTestProvider; incompatible with t.Parallel
func TestRequireTokenMissingCreds(t *testing.T) {
	p := newTestProvider(t)
	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected error when credentials are missing")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want errcode.CodeAuthRequired", err)
	}
	if !strings.Contains(err.Error(), "wcloud auth login") {
		t.Fatalf("error message should suggest login: %v", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProvider; incompatible with t.Parallel
func TestRequireTokenExpired(t *testing.T) {
	p := newTestProvider(t)
	// No refresh token, so an expired access token cannot be renewed.
	writeDevCreds(t, Credentials{
		AccessToken: "EXPIRED",
		ExpiresAt:   time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected error on expired creds")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want errcode.CodeAuthRequired", err)
	}
}

func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return path
}

//nolint:paralleltest // t.Setenv via newTestProvider; incompatible with t.Parallel
func TestRequireTokenSuccess(t *testing.T) {
	p := newTestProvider(t)
	tr := &TokenResult{
		AccessToken: "AT.LIVE.TOKEN",
		ExpiresIn:   3600,
	}
	if _, err := saveCredentials("dev", tr); err != nil {
		t.Fatalf("save: %v", err)
	}
	tok, err := p.RequireToken(context.Background())
	if err != nil {
		t.Fatalf("RequireToken: %v", err)
	}
	if tok != "AT.LIVE.TOKEN" {
		t.Fatalf("token = %q", tok)
	}
}

func newTestProviderWith(t *testing.T, cfg config.AuthConfig, client *http.Client) *Provider {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	streams, _, _, _ := iostreams.Test()
	p := NewProvider(cfg, "dev", streams, client)
	p.policy = allowlist.NewPermissiveForTesting()
	return p
}

func writeDevCreds(t *testing.T, creds Credentials) {
	t.Helper()
	path, err := config.CredentialsPath("dev")
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if mkErr := os.MkdirAll(parentDir(path), 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	buf, _ := json.Marshal(creds)
	if writeErr := os.WriteFile(path, buf, 0o600); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
}

func refreshTokenServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v1/apps/token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "refresh_token" {
			http.Error(w, "unexpected grant_type", http.StatusBadRequest)
			return
		}
		if status >= http.StatusBadRequest {
			http.Error(w, body, status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshesExpired(t *testing.T) {
	body := `{"access_token":"NEW_AT","refresh_token":"NEW_RT","token_type":"Bearer","expires_in":3600}`
	srv := refreshTokenServer(t, body, http.StatusOK)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "OLD_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	tok, err := p.RequireToken(context.Background())
	if err != nil {
		t.Fatalf("RequireToken: %v", err)
	}
	if tok != "NEW_AT" {
		t.Fatalf("token = %q, want NEW_AT", tok)
	}

	creds, err := loadCredentials("dev")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if creds.AccessToken != "NEW_AT" || creds.RefreshToken != "NEW_RT" {
		t.Fatalf("persisted creds = %+v, want rotated NEW_AT/NEW_RT", creds)
	}
	if !creds.ExpiresAt.After(time.Now()) {
		t.Fatalf("ExpiresAt = %v, want future", creds.ExpiresAt)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshKeepsTokenWhenNotRotated(t *testing.T) {
	srv := refreshTokenServer(t, `{"access_token":"NEW_AT","token_type":"Bearer","expires_in":3600}`, http.StatusOK)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "KEEP_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	if _, err := p.RequireToken(context.Background()); err != nil {
		t.Fatalf("RequireToken: %v", err)
	}
	creds, err := loadCredentials("dev")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if creds.RefreshToken != "KEEP_RT" {
		t.Fatalf("refresh token = %q, want preserved KEEP_RT", creds.RefreshToken)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshFailureFallsBackToReLogin(t *testing.T) {
	srv := refreshTokenServer(t, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "DEAD_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected error when refresh fails")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}
	if !strings.Contains(err.Error(), "wcloud auth login") {
		t.Fatalf("err should suggest re-login: %v", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshTransientErrorPropagates(t *testing.T) {
	// A 5xx is transient — the session is fine, Descope is momentarily down.
	// It must NOT be reported as auth_required (which forces a needless
	// full re-login); it should propagate as a retryable error.
	srv := refreshTokenServer(t, `{"error":"server_error"}`, http.StatusServiceUnavailable)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "GOOD_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected error on transient refresh failure")
	}
	var e *errcode.Error
	if errors.As(err, &e) && e.Code == errcode.CodeAuthRequired {
		t.Fatalf("transient failure must not be auth_required: %v", err)
	}
	if !strings.Contains(err.Error(), "refresh access token") {
		t.Fatalf("err should surface the refresh failure: %v", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshConsentRevokedRequiresReLogin(t *testing.T) {
	body := `{"errorCode":"E063302","errorMessage":"Cannot find consent by id"}`
	srv := refreshTokenServer(t, body, http.StatusNotFound)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "REVOKED_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected error when consent revoked")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}
	if !strings.Contains(err.Error(), "wcloud auth login") {
		t.Fatalf("err should suggest re-login: %v", err)
	}
	if strings.Contains(err.Error(), "E063302") {
		t.Fatalf("raw Descope body must not leak: %v", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshRateLimitedPropagates(t *testing.T) {
	srv := refreshTokenServer(t, `{"error":"rate_limited"}`, http.StatusTooManyRequests)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "GOOD_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected error on 429 refresh failure")
	}
	var e *errcode.Error
	if errors.As(err, &e) && e.Code == errcode.CodeAuthRequired {
		t.Fatalf("429 is transient, must not be auth_required: %v", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshesWithinSkew(t *testing.T) {
	// Token is not yet expired but within expirySkew of expiry — the proactive
	// refresh branch must fire so the token never dies mid-request.
	body := `{"access_token":"FRESH_AT","refresh_token":"FRESH_RT","token_type":"Bearer","expires_in":3600}`
	srv := refreshTokenServer(t, body, http.StatusOK)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "NEAR_EXPIRY_AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(30 * time.Second),
	})

	tok, err := p.RequireToken(context.Background())
	if err != nil {
		t.Fatalf("RequireToken: %v", err)
	}
	if tok != "FRESH_AT" {
		t.Fatalf("token = %q, want proactive refresh to FRESH_AT", tok)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshEmptyAccessTokenReLogin(t *testing.T) {
	// 200 OK but no access_token — treat as no usable session and prompt
	// re-login rather than returning/persisting an empty bearer token.
	srv := refreshTokenServer(t, `{"refresh_token":"X","expires_in":3600}`, http.StatusOK)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProvider; incompatible with t.Parallel
func TestRequireTokenNoExpiryReturnsTokenAsIs(t *testing.T) {
	// Credentials with no ExpiresAt are treated as non-expiring: the token is
	// returned without attempting a refresh (no server is configured here).
	p := newTestProvider(t)
	writeDevCreds(t, Credentials{AccessToken: "AT_NO_EXP"})

	tok, err := p.RequireToken(context.Background())
	if err != nil {
		t.Fatalf("RequireToken: %v", err)
	}
	if tok != "AT_NO_EXP" {
		t.Fatalf("token = %q, want AT_NO_EXP", tok)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshRetightensLoosenedCredentialsFile(t *testing.T) {
	body := `{"access_token":"NEW_AT","refresh_token":"NEW_RT","token_type":"Bearer","expires_in":3600}`
	srv := refreshTokenServer(t, body, http.StatusOK)
	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "OLD_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})
	path, err := config.CredentialsPath("dev")
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if chmodErr := os.Chmod(path, 0o644); chmodErr != nil {
		t.Fatalf("pre-loosen: %v", chmodErr)
	}

	if _, reqErr := p.RequireToken(context.Background()); reqErr != nil {
		t.Fatalf("RequireToken: %v", reqErr)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat: %v", statErr)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm after background refresh = %o, want 0600 (re-tightened)", mode)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestNewProviderHTTPClientIsBounded(t *testing.T) {
	p := newTestProviderWith(t, config.AuthConfig{}, nil)
	if p.http.Timeout != authHTTPTimeout {
		t.Fatalf("default client Timeout = %v, want %v", p.http.Timeout, authHTTPTimeout)
	}

	injected := &http.Client{Timeout: 7 * time.Second}
	q := newTestProviderWith(t, config.AuthConfig{}, injected)
	if q.http != injected {
		t.Fatal("an explicitly injected client must be used unchanged")
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenRefreshTimeoutIsTransientNotAuthRequired(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	p := newTestProviderWith(t, config.AuthConfig{BaseURL: slow.URL, ClientID: "cid"},
		&http.Client{Timeout: 50 * time.Millisecond})
	writeDevCreds(t, Credentials{
		AccessToken:  "AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected the refresh to fail")
	}
	if code, ok := errcode.CodeFor(err); ok && code == errcode.CodeAuthRequired {
		t.Fatalf("a timed-out refresh must not force a re-login, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

//nolint:paralleltest // t.Setenv via newTestProvider; incompatible with t.Parallel
func TestRequireTokenCorruptedFileErrors(t *testing.T) {
	p := newTestProvider(t)
	path, err := config.CredentialsPath("dev")
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if mkErr := os.MkdirAll(parentDir(path), 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	if wErr := os.WriteFile(path, []byte("not json"), 0o600); wErr != nil {
		t.Fatalf("write: %v", wErr)
	}

	_, reqErr := p.RequireToken(context.Background())
	if reqErr == nil {
		t.Fatal("expected error on corrupted credentials file")
	}
	// A corrupt file is a distinct condition from "never signed in", but it
	// shares the same remedy (re-login overwrites the file) and error codes
	// are a frozen contract — so it must reuse auth_required, not invent one.
	var e *errcode.Error
	if !errors.As(reqErr, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want errcode.CodeAuthRequired", reqErr)
	}
	if !strings.Contains(reqErr.Error(), "wcloud auth login") {
		t.Fatalf("error message should suggest login: %v", reqErr)
	}
	if !strings.Contains(reqErr.Error(), "corrupt") {
		t.Fatalf("error message should name the real cause (corrupt file), not a generic decode failure: %v", reqErr)
	}
	if strings.Contains(reqErr.Error(), "invalid character") {
		t.Fatalf("raw JSON parser output must not leak into the user-facing message: %v", reqErr)
	}
	if strings.Contains(reqErr.Error(), "unreadable") {
		t.Fatalf("message must not claim to handle unreadable files; only decode failures are handled: %v", reqErr)
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestRequireTokenTransportFailureCarriesFailureStage(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	dead.Close()

	p := newTestProviderWith(t, config.AuthConfig{BaseURL: dead.URL, ClientID: "CID"}, dead.Client())
	writeDevCreds(t, Credentials{
		AccessToken:  "OLD_AT",
		RefreshToken: "GOOD_RT",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	_, err := p.RequireToken(context.Background())
	if err == nil {
		t.Fatal("expected the refresh to fail")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want the chain to carry *errcode.Error", err)
	}
	if got := e.Details[errcode.DetailFailureStage]; got != errcode.FailureStageTransport {
		t.Errorf("Details[%q] = %v, want %q", errcode.DetailFailureStage, got, errcode.FailureStageTransport)
	}
	code, ok := errcode.CodeFor(err)
	if !ok || code != errcode.CodeInternalError {
		t.Errorf("code = %q (ok=%v), want %q — the code and exit must not move", code, ok, errcode.CodeInternalError)
	}
	for _, phrase := range []string{"refresh access token", "auth.weaviate.cloud", "api-cloud.weaviate.cloud"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("message is missing %q through the refresh wrap: %v", phrase, err)
		}
	}
}
