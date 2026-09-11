//nolint:testpackage // covers unexported helpers
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/config"
)

//nolint:paralleltest // t.Setenv mutates process env
func TestProviderLogoutRemovesCredentials(t *testing.T) {
	p := newTestProvider(t)

	tr := &TokenResult{AccessToken: "AT", ExpiresIn: 3600}
	path, err := saveCredentials("dev", tr)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("creds should exist before logout: %v", statErr)
	}

	if logoutErr := p.Logout(context.Background()); logoutErr != nil {
		t.Fatalf("Logout: %v", logoutErr)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("creds should be gone after logout: %v", statErr)
	}
}

//nolint:paralleltest // t.Setenv mutates process env
func TestProviderLogoutMissingFileOK(t *testing.T) {
	p := newTestProvider(t)
	if err := p.Logout(context.Background()); err != nil {
		t.Fatalf("Logout on missing file should be a no-op, got: %v", err)
	}
	_ = config.DefaultProfileName
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestProviderLogoutRevokesRefreshToken(t *testing.T) {
	var revoked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v1/apps/revoke" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.PostForm.Get("token") == "RT" &&
			r.PostForm.Get("token_type_hint") == "refresh_token" &&
			r.PostForm.Get("client_id") == "CID" &&
			r.Header.Get("Authorization") == "Bearer AT" {
			revoked = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "unexpected revoke request", http.StatusBadRequest)
	}))
	defer srv.Close()

	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{AccessToken: "AT", RefreshToken: "RT", ExpiresAt: time.Now().Add(time.Hour)})

	if err := p.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !revoked {
		t.Fatal("expected the refresh token to be revoked server-side")
	}
	path, _ := config.CredentialsPath("dev")
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("local credentials should be removed after logout")
	}
}

//nolint:paralleltest // t.Setenv via newTestProviderWith; incompatible with t.Parallel
func TestProviderLogoutRevokeFailureStillClearsLocal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newTestProviderWith(t, config.AuthConfig{BaseURL: srv.URL, ClientID: "CID"}, srv.Client())
	writeDevCreds(t, Credentials{AccessToken: "AT", RefreshToken: "RT", ExpiresAt: time.Now().Add(time.Hour)})

	if err := p.Logout(context.Background()); err != nil {
		t.Fatalf("logout must not fail when revoke fails: %v", err)
	}
	path, _ := config.CredentialsPath("dev")
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("local credentials should be removed even when revoke fails")
	}
}
