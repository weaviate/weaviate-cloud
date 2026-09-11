//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestExchangeCodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v1/apps/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("grant_type") != "authorization_code" ||
			r.PostForm.Get("code") != "AUTH_CODE" ||
			r.PostForm.Get("client_id") != "CID" ||
			r.PostForm.Get("code_verifier") != "VERIFIER" {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600,"scope":"openid"}`))
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	tr, err := exchangeCode(
		context.Background(), cfg, srv.Client(), allowlist.NewPermissiveForTesting(),
		"AUTH_CODE", "http://127.0.0.1:53682/cb", "VERIFIER",
	)
	if err != nil {
		t.Fatalf("exchangeCode: %v", err)
	}
	if tr.AccessToken != "AT" || tr.TokenType != "Bearer" || tr.ExpiresIn != 3600 {
		t.Fatalf("token = %+v", tr)
	}
}

func TestRefreshTokensSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v1/apps/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("grant_type") != "refresh_token" ||
			r.PostForm.Get("client_id") != "CID" ||
			r.PostForm.Get("refresh_token") != "RT" {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostForm.Has("client_secret") {
			http.Error(w, "public client must not send a secret", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT2","refresh_token":"RT2","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	tr, err := refreshTokens(context.Background(), cfg, srv.Client(), allowlist.NewPermissiveForTesting(), "RT")
	if err != nil {
		t.Fatalf("refreshTokens: %v", err)
	}
	if tr.AccessToken != "AT2" || tr.RefreshToken != "RT2" {
		t.Fatalf("token = %+v", tr)
	}
}

func TestExchangeCodeServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	_, err := exchangeCode(context.Background(), cfg, srv.Client(), allowlist.NewPermissiveForTesting(), "X", "Y", "Z")
	if err == nil {
		t.Fatal("expected error on 4xx, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("err = %v, want HTTP 400 mention", err)
	}
}

func TestExchangeCodeBadJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	_, err := exchangeCode(context.Background(), cfg, srv.Client(), allowlist.NewPermissiveForTesting(), "X", "Y", "Z")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode token response") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestTokenNoResponseCarriesDiscriminator(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"RT"}}
	_, err := requestToken(context.Background(), cfg, srv.Client(), allowlist.NewPermissiveForTesting(), form)

	if err == nil {
		t.Fatal("expected an error from a closed server")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errcode.Error", err)
	}
	if e.Code != errcode.CodeInternalError {
		t.Errorf("Code = %q, want %q", e.Code, errcode.CodeInternalError)
	}
	if got := e.Details[errcode.DetailFailureStage]; got != errcode.FailureStageTransport {
		t.Errorf("Details[%q] = %v, want %q", errcode.DetailFailureStage, got, errcode.FailureStageTransport)
	}
	if errors.Unwrap(e) == nil {
		t.Error("the underlying transport error must stay reachable through Unwrap")
	}
}

func TestRequestTokenNoResponseOnClientTimeout(t *testing.T) {
	t.Parallel()

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	cfg := config.AuthConfig{BaseURL: slow.URL, ClientID: "CID"}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"RT"}}
	client := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := requestToken(context.Background(), cfg, client, allowlist.NewPermissiveForTesting(), form)

	// A command that exhausted its own time budget stays in this branch: a blackholed
	// connection legitimately times out, and the message is true for both.
	var e *errcode.Error
	if !errors.As(err, &e) || e.Details[errcode.DetailFailureStage] != errcode.FailureStageTransport {
		t.Fatalf("a client timeout must stay in the transport branch, got %v (%T)", err, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline must stay reachable through the wrapped error")
	}
	if !strings.Contains(err.Error(), "auth.weaviate.cloud") {
		t.Error("the message must name the hosts on a timeout too")
	}
}

func TestRequestTokenCancelledContextIsNotDiagnosed(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"RT"}}
	_, err := requestToken(ctx, cfg, srv.Client(), allowlist.NewPermissiveForTesting(), form)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, ok := errcode.CodeFor(err); ok {
		t.Errorf("a cancelled command must not acquire an error code: %v", err)
	}
	if strings.Contains(err.Error(), "weaviate.cloud") {
		t.Errorf("a cancelled command must not be handed network-policy advice: %v", err)
	}
}
