//go:build wcloud_dev

//nolint:testpackage // covers unexported helpers
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
)

func TestRequestToken_ReachesLoopbackHostWithDevTag(t *testing.T) {
	t.Parallel()
	var handlerHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","refresh_token":"RT","expires_in":3600}`))
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"RT"}}
	tr, err := requestToken(context.Background(), cfg, srv.Client(), allowlist.Production(), form)
	if err != nil {
		t.Fatalf("requestToken: %v — a developer build must reach a loopback auth host", err)
	}
	if !handlerHit {
		t.Fatal("the loopback token handler was never invoked")
	}
	if tr.AccessToken != "AT" {
		t.Fatalf("AccessToken = %q, want AT", tr.AccessToken)
	}
}

func TestRevokeRefreshToken_ReachesLoopbackHostWithDevTag(t *testing.T) {
	t.Parallel()
	var handlerHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}
	if err := revokeRefreshToken(context.Background(), cfg, srv.Client(), allowlist.Production(), "AT", "RT"); err != nil {
		t.Fatalf("revokeRefreshToken: %v — a developer build must reach a loopback auth host", err)
	}
	if !handlerHit {
		t.Fatal("the loopback revoke handler was never invoked")
	}
}
