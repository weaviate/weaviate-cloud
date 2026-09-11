// Package cmdtest builds a fully wired root command for driving cobra
// commands end to end in tests. It is test-only: no non-test package may
// import it.
package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/auth"
	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/factory/mocks"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

const liveTokenTTLSeconds = 3600

func NewFactory(t *testing.T, signedIn bool) (*factory.Factory, *mocks.MockAPIClient, *bytes.Buffer) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	// WHY: Windows reads %AppData%/%USERPROFILE%, so without these a test writes to the contributor's real profile.
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)

	io, _, stdout, _ := iostreams.Test()
	provider := auth.NewProvider(config.AuthConfig{}, "default", io, nil)
	if signedIn {
		if _, err := provider.Save(&auth.TokenResult{AccessToken: "AT", ExpiresIn: liveTokenTTLSeconds}); err != nil {
			t.Fatalf("save creds: %v", err)
		}
	}

	apiMock := mocks.NewMockAPIClient(t)
	f := &factory.Factory{
		IOStreams:    io,
		Endpoint:     "https://example.com",
		Auth:         provider,
		NewAPIClient: func(string) factory.APIClient { return apiMock },
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "req-fixed" },
	}
	return f, apiMock, stdout
}

func Run(t *testing.T, f *factory.Factory, args ...string) error {
	t.Helper()
	root := cli.NewRootCmd(f)
	root.SetArgs(append(args, "-o", "json"))
	return root.ExecuteContext(context.Background())
}

type Envelope struct {
	Data     json.RawMessage `json:"data"`
	Metadata struct {
		APIVersion string `json:"api_version"`
		RequestID  string `json:"request_id"`
	} `json:"metadata"`
}

func DecodeEnvelope(t *testing.T, stdout *bytes.Buffer) Envelope {
	t.Helper()
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v\nstdout=%s", err, stdout.String())
	}
	if env.Metadata.APIVersion != output.APIVersion || env.Metadata.RequestID == "" {
		t.Fatalf("metadata = %+v, want api_version=%s + non-empty request_id", env.Metadata, output.APIVersion)
	}
	return env
}
