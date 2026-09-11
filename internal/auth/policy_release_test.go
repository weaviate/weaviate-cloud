//go:build !wcloud_dev

// WHY: these tests pin the release build's credential-destination policy — each points a credential-carrying call at an httptest server, a loopback host that allowlist.Production() refuses in a release build and deliberately permits under -tags wcloud_dev, so they can only assert refusal here; the developer build's permit side is policy_dev_test.go (requestToken, revokeRefreshToken) and oauth_dev_test.go (runLogin).

//nolint:testpackage // needs config.ResolveWith + the real, unexported Provider wiring
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func TestRequireToken_EnvOverrideRefusesRefreshTokenToAttackerHost(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	var handlerHit bool
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer attacker.Close()

	t.Setenv(config.AuthBaseURLEnvVar, attacker.URL)

	profiles, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	active := config.ActiveProfile(profiles)
	if active != config.DefaultProfileName {
		t.Fatalf(
			"active profile = %q, want %q (env override must not switch profiles)",
			active, config.DefaultProfileName,
		)
	}
	cfg := config.ResolveWith(profiles)
	if cfg.Auth.BaseURL != attacker.URL {
		t.Fatalf("cfg.Auth.BaseURL = %q, want the attacker URL %q", cfg.Auth.BaseURL, attacker.URL)
	}

	streams, _, _, _ := iostreams.Test()
	provider := NewProvider(cfg.Auth, active, streams, nil)

	path, err := config.CredentialsPath(active)
	if err != nil {
		t.Fatalf("credentials path: %v", err)
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	expired := `{"access_token":"PROD_AT","refresh_token":"PROD_RT","expires_at":"` +
		time.Now().Add(-time.Hour).Format(time.RFC3339) + `"}`
	if wErr := os.WriteFile(path, []byte(expired), 0o600); wErr != nil {
		t.Fatalf("write creds: %v", wErr)
	}

	_, tokenErr := provider.RequireToken(context.Background())
	if tokenErr == nil {
		t.Fatal("expected RequireToken to fail")
	}
	var e *errcode.Error
	if !errors.As(tokenErr, &e) || e.Code != errcode.CodeValidationFailed {
		t.Fatalf("err = %v, want errcode.CodeValidationFailed", tokenErr)
	}
	if handlerHit {
		t.Fatal("PROVEN BUG WOULD REPRODUCE: attacker host received the refresh POST")
	}

	onDisk, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read creds: %v", readErr)
	}
	if !strings.Contains(string(onDisk), "PROD_RT") || strings.Contains(string(onDisk), "ATTACKER") {
		t.Fatalf("on-disk credentials were modified: %s", onDisk)
	}
}

func TestRequestToken_RefusesCredentialToNonAllowedHost(t *testing.T) {
	t.Parallel()
	var handlerHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		handlerHit = true
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"SECRET_RT"}}
	_, err := requestToken(context.Background(), cfg, srv.Client(), allowlist.Production(), form)

	if err == nil {
		t.Fatal("expected an error; srv.URL is a loopback host, not on the allowlist")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeValidationFailed {
		t.Fatalf("err = %v, want errcode.CodeValidationFailed", err)
	}
	if handlerHit {
		t.Fatal("the disallowed host's handler must never be invoked")
	}
	if strings.Contains(err.Error(), "SECRET_RT") {
		t.Fatalf("error message leaks the refresh token: %v", err)
	}
}

func TestRevokeRefreshToken_RefusesCredentialToNonAllowedHost(t *testing.T) {
	t.Parallel()
	var handlerHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		handlerHit = true
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	err := revokeRefreshToken(context.Background(), cfg, srv.Client(), allowlist.Production(), "SECRET_AT", "SECRET_RT")

	if err == nil {
		t.Fatal("expected an error; srv.URL is a loopback host, not on the allowlist")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeValidationFailed {
		t.Fatalf("err = %v, want errcode.CodeValidationFailed", err)
	}
	if handlerHit {
		t.Fatal("the disallowed host's handler must never be invoked")
	}
	if strings.Contains(err.Error(), "SECRET_AT") || strings.Contains(err.Error(), "SECRET_RT") {
		t.Fatalf("error message leaks a credential: %v", err)
	}
}

func TestAuthClientRefusesRedirectOffTheAllowlist(t *testing.T) {
	t.Parallel()

	received := make(chan string, 1)
	sink := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		received <- string(buf[:n])
	}))
	t.Cleanup(sink.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	p := NewProvider(config.AuthConfig{}, "", nil, nil)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, origin.URL,
		strings.NewReader("grant_type=refresh_token&refresh_token=SENTINEL_LEAK"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if _, doErr := p.http.Do(req); doErr == nil {
		t.Fatal("expected the off-allowlist redirect to be refused")
	}

	select {
	case body := <-received:
		t.Fatalf("credential reached the redirect target: %q", body)
	default:
	}
}
