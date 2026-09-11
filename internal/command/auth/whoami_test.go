package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestWhoamiRequiresAuth(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, false)
	err := cmdtest.Run(t, f, "auth", "whoami")
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}

	path, pathErr := config.CredentialsPath("default")
	if pathErr != nil {
		t.Fatalf("path: %v", pathErr)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no credentials file after a read-only auth-required command, stat err = %v", statErr)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestWhoamiRetightensLoosenedCredentialsFile(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	who := &api.WhoAmI{UserID: "u1", Email: "u1@example.com", OrgID: "o1"}
	apiMock.EXPECT().Whoami(mock.Anything).Return(who, nil)

	path, err := config.CredentialsPath("default")
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if chmodErr := os.Chmod(path, 0o644); chmodErr != nil {
		t.Fatalf("pre-loosen: %v", chmodErr)
	}

	if runErr := cmdtest.Run(t, f, "auth", "whoami"); runErr != nil {
		t.Fatalf("whoami: %v", runErr)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat: %v", statErr)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm after whoami = %o, want 0600 (re-tightened)", mode)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestWhoamiCallsAPI(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	who := &api.WhoAmI{UserID: "u1", Email: "u1@example.com", OrgID: "o1"}
	apiMock.EXPECT().Whoami(mock.Anything).Return(who, nil)

	if err := cmdtest.Run(t, f, "auth", "whoami"); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got api.WhoAmI
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if got.Email != "u1@example.com" {
		t.Fatalf("email = %q, want u1@example.com", got.Email)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestWhoamiTextOutput(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	who := &api.WhoAmI{UserID: "u1", Email: "u1@example.com", OrgID: "o1"}
	apiMock.EXPECT().Whoami(mock.Anything).Return(who, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "auth", "whoami"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "user_id:") || !strings.Contains(got, "email:") || !strings.Contains(got, "org_id:") {
		t.Fatalf("expected key-value pairs, got %q", got)
	}
	if !strings.Contains(got, "u1@example.com") {
		t.Fatalf("expected email value, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}
