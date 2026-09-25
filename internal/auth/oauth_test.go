//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

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

//nolint:paralleltest // shares the loopback port pool with sibling tests
func TestRunLoginTokenExchangeFailureClassification(t *testing.T) {
	cases := []struct {
		name             string
		status           int
		body             string
		wantAuthRequired bool
	}{
		{"rejected grant (400) is auth_required", http.StatusBadRequest, `{"error":"invalid_grant"}`, true},
		{"rate limited (429) is not auth_required", http.StatusTooManyRequests, `{"error":"rate_limited"}`, false},
		{"server error (503) is not auth_required", http.StatusServiceUnavailable, `{"error":"server_error"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runLoginAgainstTokenResponse(t, tc.status, tc.body)
			if err == nil {
				t.Fatal("expected an error from the rejected token exchange")
			}
			var ec *errcode.Error
			isAuthRequired := errors.As(err, &ec) && ec.Code == errcode.CodeAuthRequired
			if isAuthRequired != tc.wantAuthRequired {
				t.Fatalf("auth_required = %v, want %v (err = %v)", isAuthRequired, tc.wantAuthRequired, err)
			}
			if tc.wantAuthRequired {
				if got := errcode.ExitCodeFor(err); got != errcode.AuthRequired {
					t.Fatalf("exit code = %d, want %d", got, errcode.AuthRequired)
				}
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", tc.status)) {
				t.Fatalf("message should keep the HTTP status for diagnosis, got %q", err.Error())
			}
		})
	}
}

func runLoginAgainstTokenResponse(t *testing.T, status int, body string) error {
	t.Helper()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, body, status)
	}))
	defer tokenSrv.Close()

	errBuf := &syncBuffer{}
	streams := &iostreams.IOStreams{In: strings.NewReader(""), Out: &strings.Builder{}, Err: errBuf}
	cfg := config.AuthConfig{BaseURL: tokenSrv.URL, ClientID: "CID"}

	resultCh := make(chan error, 1)
	go func() {
		_, err := runLogin(
			context.Background(), cfg, tokenSrv.Client(), allowlist.NewPermissiveForTesting(),
			streams, LoginOptions{Timeout: 5 * time.Second, NoLaunchBrowser: true, Now: time.Now},
		)
		resultCh <- err
	}()

	authURL := waitForSignInURL(t, errBuf)
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	state := u.Query().Get("state")
	redirectURI := u.Query().Get("redirect_uri")
	if state == "" || redirectURI == "" {
		t.Fatalf("authorize URL missing state/redirect_uri: %s", authURL)
	}

	callbackURL := redirectURI + "?state=" + url.QueryEscape(state) + "&code=AUTH_CODE"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, callbackURL, http.NoBody)
	if err != nil {
		t.Fatalf("build callback request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deliver callback: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case loginErr := <-resultCh:
		return loginErr
	case <-time.After(5 * time.Second):
		t.Fatal("runLogin did not return after the callback was delivered")
		return nil
	}
}

func waitForSignInURL(t *testing.T, errBuf *syncBuffer) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s := errBuf.String(); strings.Contains(s, "http") {
			for line := range strings.SplitSeq(s, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "http") {
					return line
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for the sign-in URL on stderr, got %q", errBuf.String())
	return ""
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
