//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func TestBuildAuthorizeURL(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		BaseURL:  "https://auth.example.com",
		ClientID: "test-client",
	}
	raw := buildAuthorizeURL(cfg, "http://127.0.0.1:53682/callback", "state-123", "chal-abc")

	if !strings.HasPrefix(raw, "https://auth.example.com/oauth2/v1/apps/authorize?") {
		t.Fatalf("URL does not target apps/authorize: %s", raw)
	}

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]string{
		"response_type":         "code",
		"client_id":             "test-client",
		"redirect_uri":          "http://127.0.0.1:53682/callback",
		"scope":                 defaultScope,
		"state":                 "state-123",
		"code_challenge":        "chal-abc",
		"code_challenge_method": "S256",
	}
	for k, v := range want {
		if got := u.Query().Get(k); got != v {
			t.Errorf("query %s = %q, want %q", k, got, v)
		}
	}
}

func TestRunLoginTimesOutWithStructuredDetails(t *testing.T) {
	t.Parallel()

	streams, _, _, errBuf := iostreams.Test()
	cfg := config.AuthConfig{BaseURL: "https://auth.example.com", ClientID: "test-client"}

	start := time.Now()
	_, err := runLogin(
		context.Background(),
		cfg,
		http.DefaultClient,
		allowlist.NewPermissiveForTesting(),
		streams,
		LoginOptions{Timeout: 150 * time.Millisecond, NoLaunchBrowser: true, Now: time.Now},
	)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("returned after %v, want at least the 150ms deadline", elapsed)
	}
	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("err is %T, want *errcode.Error", err)
	}
	if ec.Code != errcode.CodeAuthRequired {
		t.Fatalf("code = %q, want %q", ec.Code, errcode.CodeAuthRequired)
	}
	if got, _ := ec.Details["waited"].(string); got != "150ms" {
		t.Fatalf("details[waited] = %v, want \"150ms\"", ec.Details["waited"])
	}
	if !strings.Contains(ec.Message, "--timeout") {
		t.Fatalf("message must name --timeout, got %q", ec.Message)
	}
	if strings.Contains(ec.Message, "https://") {
		t.Fatalf("the URL from this run is spent once the listener closes; the remedy must not name it, got %q",
			ec.Message)
	}
	if !strings.Contains(ec.Message, "wcloud auth login") {
		t.Fatalf("the remedy must be to re-run the command, got %q", ec.Message)
	}
	if !strings.Contains(errBuf.String(), "https://auth.example.com/oauth2/v1/apps/authorize?") {
		t.Fatalf("stderr must carry the sign-in URL, got %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "Not opening a browser (--no-launch-browser).") {
		t.Fatalf("stderr must record that the opener was skipped, got %q", errBuf.String())
	}
}

// WHY: the sign-in URL was ungated even though the token exchange already was — an attacker page could harvest the password and MFA before any token moved.
func TestRunLoginRefusesAnAuthorizeURLOutsideTheAllowlist(t *testing.T) {
	t.Parallel()

	streams, _, _, errBuf := iostreams.Test()
	cfg := config.AuthConfig{BaseURL: "https://auth.evil.example", ClientID: "test-client"}

	_, err := runLogin(
		context.Background(),
		cfg,
		http.DefaultClient,
		allowlist.Production(),
		streams,
		LoginOptions{Timeout: 150 * time.Millisecond, NoLaunchBrowser: true, Now: time.Now},
	)

	if err == nil {
		t.Fatal("expected the sign-in URL to be refused")
	}
	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("err is %T, want *errcode.Error", err)
	}
	if ec.Code != errcode.CodeValidationFailed {
		t.Fatalf("code = %q, want %q", ec.Code, errcode.CodeValidationFailed)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("nothing may reach the user before the URL is cleared, got %q", errBuf.String())
	}
}

func TestRunLoginAcceptsThePermittedAuthorizeURL(t *testing.T) {
	t.Parallel()

	streams, _, _, errBuf := iostreams.Test()
	cfg := config.AuthConfig{BaseURL: "https://auth.weaviate.cloud", ClientID: "test-client"}

	_, err := runLogin(
		context.Background(),
		cfg,
		http.DefaultClient,
		allowlist.Production(),
		streams,
		LoginOptions{Timeout: 150 * time.Millisecond, NoLaunchBrowser: true, Now: time.Now},
	)

	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("err is %T, want *errcode.Error", err)
	}
	if ec.Code != errcode.CodeAuthRequired {
		t.Fatalf("code = %q, want the timeout %q — the permitted host must not be gated",
			ec.Code, errcode.CodeAuthRequired)
	}
	if !strings.Contains(errBuf.String(), "https://auth.weaviate.cloud/oauth2/v1/apps/authorize?") {
		t.Fatalf("stderr must carry the sign-in URL, got %q", errBuf.String())
	}
}

func TestAuthorizeAndTokenURLs(t *testing.T) {
	t.Parallel()

	const base = "https://auth.example.com"
	if got := authorizeURL(base); got != base+"/oauth2/v1/apps/authorize" {
		t.Fatalf("authorizeURL = %q", got)
	}
	if got := tokenURL(base); got != base+"/oauth2/v1/apps/token" {
		t.Fatalf("tokenURL = %q", got)
	}
}
