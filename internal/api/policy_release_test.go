//go:build !wcloud_dev

package api_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestClient_RefusesCredentialToNonAllowedHost(t *testing.T) {
	t.Parallel()
	var handlerHit bool
	srv := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
		handlerHit = true
	})

	// WHY: not using newTestClient here — this test exercises the real, strict, default policy.
	c := api.NewClient(srv.URL, "SECRET_TOKEN", api.WithRequestIDFunc(func() string { return "r" }))

	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected an error; srv.URL is a loopback host, not on the allowlist")
	}
	var apiErr *api.Error
	if !errors.As(err, &apiErr) || apiErr.Code != errcode.CodeValidationFailed {
		t.Fatalf("err = %v, want *api.Error with code %q", err, errcode.CodeValidationFailed)
	}
	if handlerHit {
		t.Fatal("the disallowed host's handler must never be invoked")
	}
	if strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatalf("error message leaks the credential: %v", err)
	}
}
