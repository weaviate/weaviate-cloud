package profile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func newTestFactory(t *testing.T, stdin string) (*factory.Factory, *bytes.Buffer) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	io, in, out, _ := iostreams.Test()
	in.WriteString(stdin)
	// Reset env vars so test runs aren't tainted by ambient values.
	for _, key := range []string{
		config.EndpointEnvVar,
		config.AuthBaseURLEnvVar,
		config.AuthClientIDEnvVar,
	} {
		t.Setenv(key, "")
	}

	f := &factory.Factory{
		IOStreams:    io,
		Endpoint:     config.ProductionEndpoint,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "req-fixed" },
	}
	return f, out
}

func runWithFactory(t *testing.T, f *factory.Factory, args ...string) error {
	t.Helper()
	root := cli.NewRootCmd(f)
	root.SetArgs(args)
	root.SetIn(f.IOStreams.In)
	root.SetOut(f.IOStreams.Out)
	root.SetErr(f.IOStreams.Err)
	return root.ExecuteContext(context.Background())
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileListEmpty(t *testing.T) {
	f, stdout := newTestFactory(t, "")
	if err := runWithFactory(t, f, "profile", "list", "-o", "json"); err != nil {
		t.Fatalf("profile list: %v", err)
	}

	var env struct {
		Data struct {
			Active   string `json:"active"`
			Profiles []struct {
				Name   string `json:"name"`
				Active bool   `json:"active"`
			} `json:"profiles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v\nstdout=%s", err, stdout.String())
	}
	if env.Data.Active != "default" {
		t.Fatalf("active = %q, want default", env.Data.Active)
	}
	if len(env.Data.Profiles) != 1 || env.Data.Profiles[0].Name != "default" {
		t.Fatalf("profiles = %+v, want only [default]", env.Data.Profiles)
	}
	if !env.Data.Profiles[0].Active {
		t.Fatalf("default should be marked active")
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileCreateRejectsNonTTY(t *testing.T) {
	f, _ := newTestFactory(t, "")
	err := runWithFactory(t, f, "profile", "create", "dev", "-o", "json")
	if err == nil {
		t.Fatal("expected error on non-TTY stdin")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %q, want interactive terminal message", err)
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileUseUnknownErrors(t *testing.T) {
	f, _ := newTestFactory(t, "")
	err := runWithFactory(t, f, "profile", "use", "ghost", "-o", "json")
	if err == nil {
		t.Fatal("expected error using unknown profile")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("error = %q, want does-not-exist", err)
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileUseSwitchesToDefault(t *testing.T) {
	f, stdout := newTestFactory(t, "")

	// Seed a profile by saving via the config package.
	in := config.Profiles{
		Active: "default",
		Profiles: map[string]config.Profile{
			"dev": {Endpoint: "https://api.dev.example.com"},
		},
	}
	if err := config.SaveProfiles(in); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	if err := runWithFactory(t, f, "profile", "use", "dev", "-o", "json"); err != nil {
		t.Fatalf("profile use: %v", err)
	}

	got, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if got.Active != "dev" {
		t.Fatalf("active after use = %q, want dev. stdout=%s", got.Active, stdout.String())
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileDeleteDefaultRejected(t *testing.T) {
	f, _ := newTestFactory(t, "y\n")
	err := runWithFactory(t, f, "profile", "delete", "default", "-o", "json")
	if err == nil {
		t.Fatal("expected error deleting default")
	}
	if !strings.Contains(err.Error(), "built-in") {
		t.Fatalf("error = %q, want built-in message", err)
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileShowRedactsCredentials(t *testing.T) {
	f, stdout := newTestFactory(t, "")

	in := config.Profiles{
		Active: "dev",
		Profiles: map[string]config.Profile{
			"dev": {
				Endpoint: "https://api.dev.example.com",
				Auth: &config.AuthConfig{
					BaseURL:  "https://auth.example.com",
					ClientID: "SECRET_C",
				},
			},
		},
	}
	if err := config.SaveProfiles(in); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	if err := runWithFactory(t, f, "profile", "show", "dev", "-o", "json"); err != nil {
		t.Fatalf("profile show: %v", err)
	}

	output := stdout.String()
	for _, leak := range []string{"SECRET_C", "https://auth.example.com", "https://api.dev.example.com"} {
		if strings.Contains(output, leak) {
			t.Fatalf("output contains leaked value %q: %s", leak, output)
		}
	}

	var env struct {
		Data struct {
			Name               string `json:"name"`
			Active             bool   `json:"active"`
			EndpointConfigured bool   `json:"endpoint_configured"`
			AuthConfigured     bool   `json:"auth_configured"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v\nstdout=%s", err, output)
	}
	if !env.Data.Active || !env.Data.EndpointConfigured || !env.Data.AuthConfigured {
		t.Fatalf("flags wrong: %+v", env.Data)
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileListIsHiddenFromHelp(t *testing.T) {
	f, _ := newTestFactory(t, "")
	rootHelp := bytes.Buffer{}
	root := cli.NewRootCmd(f)
	root.SetOut(&rootHelp)
	if err := root.Help(); err != nil {
		t.Fatalf("Help: %v", err)
	}
	if strings.Contains(rootHelp.String(), "profile") {
		t.Fatalf("profile must be hidden from root help, got:\n%s", rootHelp.String())
	}
}

func TestCredentialsPathRespectsProfile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	defPath, err := config.CredentialsPath(config.DefaultProfileName)
	if err != nil {
		t.Fatalf("CredentialsPath(default): %v", err)
	}
	if !strings.HasSuffix(defPath, "credentials.json") {
		t.Fatalf("default path = %q", defPath)
	}
	customPath, err := config.CredentialsPath("dev")
	if err != nil {
		t.Fatalf("CredentialsPath(dev): %v", err)
	}
	if !strings.HasSuffix(customPath, "credentials.dev.json") {
		t.Fatalf("dev path = %q", customPath)
	}
}

func TestDeleteCredentialsMissingFileOK(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := config.DeleteCredentials("nonexistent"); err != nil {
		t.Fatalf("DeleteCredentials should be a no-op on missing file, got: %v", err)
	}
}

func TestDeleteCredentialsRemovesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	path, err := config.CredentialsPath("dev")
	if err != nil {
		t.Fatalf("CredentialsPath: %v", err)
	}
	if mkErr := os.MkdirAll(stripBase(path), 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	if writeErr := os.WriteFile(path, []byte("{}"), 0o600); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}

	if delErr := config.DeleteCredentials("dev"); delErr != nil {
		t.Fatalf("DeleteCredentials: %v", delErr)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("file still exists after DeleteCredentials")
	}
}

func stripBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return path
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileListTextOutput(t *testing.T) {
	f, stdout := newTestFactory(t, "")
	if err := runWithFactory(t, f, "--output", "text", "profile", "list"); err != nil {
		t.Fatalf("profile list: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "ACTIVE") {
		t.Fatalf("expected table header NAME/ACTIVE, got %q", got)
	}
	if !strings.Contains(got, "default") {
		t.Fatalf("expected default profile row, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileShowTextOutput(t *testing.T) {
	f, stdout := newTestFactory(t, "")

	in := config.Profiles{
		Active: "dev",
		Profiles: map[string]config.Profile{
			"dev": {
				Endpoint: "https://api.dev.example.com",
				Auth: &config.AuthConfig{
					BaseURL:  "https://auth.example.com",
					ClientID: "CLIENT_ID",
				},
			},
		},
	}
	if err := config.SaveProfiles(in); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	if err := runWithFactory(t, f, "--output", "text", "profile", "show", "dev"); err != nil {
		t.Fatalf("profile show: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "name:") || !strings.Contains(got, "active:") {
		t.Fatalf("expected key-value output with name/active, got %q", got)
	}
	if !strings.Contains(got, "endpoint_configured:") || !strings.Contains(got, "auth_configured:") {
		t.Fatalf("expected endpoint_configured/auth_configured keys, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
	for _, leak := range []string{"CLIENT_ID", "https://auth.example.com", "https://api.dev.example.com"} {
		if strings.Contains(got, leak) {
			t.Fatalf("output contains leaked value %q: %s", leak, got)
		}
	}
}

//nolint:paralleltest // command tests mutate process env via t.Setenv
func TestProfileUseTextOutput(t *testing.T) {
	f, stdout := newTestFactory(t, "")

	in := config.Profiles{
		Profiles: map[string]config.Profile{
			"dev": {Endpoint: "https://api.dev.example.com"},
		},
	}
	if err := config.SaveProfiles(in); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	if err := runWithFactory(t, f, "--output", "text", "profile", "use", "dev"); err != nil {
		t.Fatalf("profile use: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "active:") || !strings.Contains(got, "dev") {
		t.Fatalf("expected key-value active: dev, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}
