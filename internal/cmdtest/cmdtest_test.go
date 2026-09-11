package cmdtest_test

import (
	"os"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
)

// [os.UserConfigDir] reads %AppData% and [os.UserHomeDir] reads %USERPROFILE% on
// Windows, so HOME/XDG_CONFIG_HOME alone let a Windows contributor's real
// credentials.json and agent-skill directory be overwritten by `go test ./...`.
//
//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestNewFactoryIsolatesEveryConfigRoot(t *testing.T) {
	cmdtest.NewFactory(t, false)

	sandbox := os.Getenv("HOME")
	if sandbox == "" {
		t.Fatal("HOME is unset; the sandbox root is undetermined")
	}
	for _, key := range []string{"XDG_CONFIG_HOME", "AppData", "USERPROFILE"} {
		if got := os.Getenv(key); got != sandbox {
			t.Errorf("%s = %q, want the sandbox root %q", key, got, sandbox)
		}
	}
}
