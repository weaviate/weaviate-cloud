//go:build wcloud_dev

package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/api"
)

func TestClient_ReachesLoopbackHostWithDevTag(t *testing.T) {
	t.Parallel()
	var handlerHit bool
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		handlerHit = true
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "r")
	})

	c := api.NewClient(srv.URL, "SECRET_TOKEN", api.WithRequestIDFunc(func() string { return "r" }))

	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("whoami: %v — a developer build must reach a loopback endpoint", err)
	}
	if !handlerHit {
		t.Fatal("the loopback handler was never invoked")
	}
}
