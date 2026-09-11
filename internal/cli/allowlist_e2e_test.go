package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/auth"
	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

// newRealFactory duplicates factory.New()'s wiring with a test IOStreams in
// place of iostreams.System() (factory.New offers no seam to override it —
// changing that is out of scope for this change). Every other line is the
// real, unmodified production path: config.ResolveWith reading real env
// vars, auth.NewProvider and api.NewClient with no policy override.
func newRealFactory(t *testing.T) *factory.Factory {
	t.Helper()
	profiles, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	active := config.ActiveProfile(profiles)
	cfg := config.ResolveWith(profiles)
	streams, _, _, _ := iostreams.Test()

	f := &factory.Factory{
		IOStreams:    streams,
		Endpoint:     cfg.Endpoint,
		Auth:         auth.NewProvider(cfg.Auth, active, streams, nil),
		Now:          time.Now,
		NewRequestID: func() string { return "req-fixed" },
	}
	f.NewAPIClient = func(token string) factory.APIClient {
		return api.NewClient(f.Endpoint, token, api.WithRequestIDFunc(f.NewRequestID))
	}
	return f
}

func TestRealFactory_RefusesCredentialWhenEndpointEnvPointsOffAllowlist(t *testing.T) {
	if allowlist.Production().Check("http://127.0.0.1:1/") == nil {
		t.Skip("developer build permits loopback; release policy is asserted in internal/api")
	}
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

	t.Setenv(config.EndpointEnvVar, attacker.URL)

	credPath, err := config.CredentialsPath(config.DefaultProfileName)
	if err != nil {
		t.Fatalf("credentials path: %v", err)
	}
	if mkErr := os.MkdirAll(filepath.Dir(credPath), 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	if wErr := os.WriteFile(credPath, []byte(`{"access_token":"FAKE_AT"}`), 0o600); wErr != nil {
		t.Fatalf("write creds: %v", wErr)
	}

	f := newRealFactory(t)
	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"region", "list", "-o", "json"})

	runErr := root.ExecuteContext(context.Background())
	if runErr == nil {
		t.Fatal("expected the command to fail")
	}
	code, ok := errcode.CodeFor(runErr)
	if !ok || code != errcode.CodeValidationFailed {
		t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeValidationFailed)
	}
	if handlerHit {
		t.Fatal("attacker host must never receive a request")
	}
}
