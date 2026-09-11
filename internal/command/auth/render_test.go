package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
)

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestLogoutTextOutput(t *testing.T) {
	f, _, stdout := cmdtest.NewFactory(t, true)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "auth", "logout"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "signed_out:") || !strings.Contains(got, "true") {
		t.Fatalf("expected signed_out: true, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}
