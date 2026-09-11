//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/config"
)

func TestAuthClientCapsRedirectHops(t *testing.T) {
	t.Parallel()

	var hops int
	loop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(loop.Close)

	p := NewProvider(config.AuthConfig{}, "", nil, nil)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, loop.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if _, doErr := p.http.Do(req); doErr == nil {
		t.Fatal("expected the redirect loop to be stopped")
	}
	if hops > maxAuthRedirects+1 {
		t.Errorf("followed %d hops, want at most %d", hops, maxAuthRedirects+1)
	}
}
