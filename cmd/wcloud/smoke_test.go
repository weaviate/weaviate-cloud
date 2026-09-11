package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestBinarySmoke(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds the wcloud binary; skipped under -short")
	}

	bin := buildBinary(t)

	cases := []struct {
		name     string
		args     []string
		wantExit int
		check    func(t *testing.T, stdout, stderr string)
	}{
		{
			name:     "version flag",
			args:     []string{"--version"},
			wantExit: errcode.Success,
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "wcloud") {
					t.Fatalf("stdout must carry the version line, got %q", stdout)
				}
				if stderr != "" {
					t.Fatalf("stderr must stay empty on a successful --version, got %q", stderr)
				}
			},
		},
		{
			name:     "unknown command",
			args:     []string{"--output", "json", "frobnicate"},
			wantExit: errcode.UsageError,
			check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				assertUsageErrorEnvelope(t, stdout)
			},
		},
		{
			name:     "unknown flag",
			args:     []string{"--output", "json", "--bogus"},
			wantExit: errcode.UsageError,
			check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				assertUsageErrorEnvelope(t, stdout)
			},
		},
		{
			name:     "group without a verb",
			args:     []string{"--output", "json", "cluster"},
			wantExit: errcode.Success,
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if stdout != "" {
					t.Fatalf("a JSON consumer must get a clean stdout, got help text: %q", stdout)
				}
				if !strings.Contains(stderr, "Usage:") {
					t.Fatalf("stderr must carry the help text, got %q", stderr)
				}
			},
		},
		{
			name:     "group with an unknown verb",
			args:     []string{"--output", "json", "cluster", "delete", "cid-1"},
			wantExit: errcode.UsageError,
			check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				assertUsageErrorEnvelope(t, stdout)
			},
		},
		{
			name:     "missing positional argument",
			args:     []string{"--output", "json", "cluster", "get"},
			wantExit: errcode.UsageError,
			check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				assertUsageErrorEnvelope(t, stdout)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, exit := runBinary(t, bin, tc.args...)
			if exit != tc.wantExit {
				t.Fatalf("exit = %d, want %d\nstdout=%q\nstderr=%q", exit, tc.wantExit, stdout, stderr)
			}
			tc.check(t, stdout, stderr)
		})
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH, cannot build the binary: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "wcloud")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, buildErr := exec.Command(goTool, "build", "-o", bin, ".").CombinedOutput(); buildErr != nil {
		t.Fatalf("build wcloud: %v\n%s", buildErr, out)
	}
	return bin
}

func runBinary(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Env = isolatedEnv(t.TempDir())
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exit := errcode.Success
	var exitErr *exec.ExitError
	switch err := cmd.Run(); {
	case err == nil:
	case errors.As(err, &exitErr):
		exit = exitErr.ExitCode()
	default:
		t.Fatalf("run %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), exit
}

// isolatedEnv keeps the run away from the contributor's real credentials and
// profiles: HOME/XDG_CONFIG_HOME on Unix, AppData/USERPROFILE on Windows.
func isolatedEnv(home string) []string {
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "HOME", "XDG_CONFIG_HOME", "AppData", "USERPROFILE":
			continue
		}
		if strings.HasPrefix(name, "WCLOUD_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+home,
		"AppData="+home,
		"USERPROFILE="+home,
	)
}

func assertUsageErrorEnvelope(t *testing.T, stdout string) {
	t.Helper()
	var env struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Metadata struct {
			APIVersion string `json:"api_version"`
			RequestID  string `json:"request_id"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not parseable JSON: %v\nraw=%q", err, stdout)
	}
	if env.Error == nil {
		t.Fatalf("envelope has no error object: %q", stdout)
	}
	if env.Error.Code != errcode.CodeValidationFailed {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, errcode.CodeValidationFailed)
	}
	if env.Error.Message == "" {
		t.Fatalf("error.message is empty: %q", stdout)
	}
	if env.Metadata.APIVersion == "" || env.Metadata.RequestID == "" {
		t.Fatalf("metadata = %+v, want non-empty api_version and request_id", env.Metadata)
	}
}
