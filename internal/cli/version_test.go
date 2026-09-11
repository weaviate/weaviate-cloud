package cli_test

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func TestVersionCommand_EmitsJSONEnvelope(t *testing.T) {
	t.Parallel()
	ios, _, out, errBuf := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("expected stderr empty, got %q", errBuf.String())
	}

	var decoded struct {
		Data struct {
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			GoVersion string `json:"go_version"`
		} `json:"data"`
		Metadata struct {
			APIVersion string `json:"api_version"`
			RequestID  string `json:"request_id"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal stdout: %v\nstdout: %s", err, out.String())
	}

	if decoded.Data.Version != cli.Version {
		t.Errorf("version = %q, want %q", decoded.Data.Version, cli.Version)
	}
	if decoded.Data.GoVersion != runtime.Version() {
		t.Errorf("go_version = %q, want %q", decoded.Data.GoVersion, runtime.Version())
	}
	if decoded.Metadata.APIVersion != cli.APIVersion {
		t.Errorf("api_version = %q, want %q", decoded.Metadata.APIVersion, cli.APIVersion)
	}
	if decoded.Metadata.RequestID != "test-request-id" {
		t.Errorf("request_id = %q, want test-request-id", decoded.Metadata.RequestID)
	}
}

// TestVersionIsLinkerStampable asserts Version is a var, not a const: a const
// cannot be stamped by `go build -ldflags -X`, and taking its address does not
// compile. Release binaries would otherwise all report the dev version.
func TestVersionIsLinkerStampable(t *testing.T) {
	t.Parallel()
	if p := &cli.Version; *p == "" {
		t.Fatal("Version is empty")
	}
}

// TestResolvedVersion_PrefersStampedVersionOverBuildInfo is NOT parallel: it
// mutates the package-global cli.Version; running sequentially avoids a data
// race with the parallel tests that read the same global.
func TestResolvedVersion_PrefersStampedVersionOverBuildInfo(t *testing.T) { //nolint:paralleltest // WHY: mutates global
	original := cli.Version
	cli.Version = "v1.2.3" //nolint:reassign // test-only: simulates a goreleaser ldflags stamp
	t.Cleanup(func() {
		cli.Version = original //nolint:reassign // test-only: restoring after the stamp simulation
	})

	if got := cli.ResolvedVersionForTesting(); got != "v1.2.3" {
		t.Errorf("resolvedVersion() = %q, want %q", got, "v1.2.3")
	}
}

// TestResolvedVersion_FallsBackToSentinelWithoutUsableBuildInfo asserts the
// fallback order's other end: with cli.Version left at its sentinel, and no
// build info available (true under `go test`, where the main module reports
// "(devel)"), resolvedVersion returns the sentinel rather than an empty or
// garbage value.
func TestResolvedVersion_FallsBackToSentinelWithoutUsableBuildInfo(t *testing.T) {
	t.Parallel()
	if cli.Version != "0.1.0-dev" {
		t.Skipf("cli.Version = %q, not the sentinel — another test left it mutated", cli.Version)
	}
	if got := cli.ResolvedVersionForTesting(); got != "0.1.0-dev" {
		t.Errorf("resolvedVersion() = %q, want sentinel %q", got, "0.1.0-dev")
	}
}

func TestRootCommand_RejectsUnsupportedOutput(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "yaml", "version"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for --output yaml, got nil")
	}
}

func TestVersionCommand_TextOutput(t *testing.T) {
	t.Parallel()
	ios, _, out, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "version:") {
		t.Fatalf("expected version: key, got %q", got)
	}
	if !strings.Contains(got, cli.Version) {
		t.Fatalf("expected version value %q in output, got %q", cli.Version, got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}
