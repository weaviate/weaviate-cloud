package factory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
)

func TestNew_defaultPollInterval(t *testing.T) {
	t.Parallel()
	f, err := factory.New("0.0.0-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if f.PollInterval != 10*time.Second {
		t.Fatalf("PollInterval = %v, want 10s", f.PollInterval)
	}
}

func TestNew_CorruptProfilesFile_FailsInsteadOfFallingBackToProduction(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("user config dir: %v", err)
	}
	if mkErr := os.MkdirAll(filepath.Join(dir, "wcloud"), 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	path := filepath.Join(dir, "wcloud", config.ProfilesFileName)
	if wErr := os.WriteFile(path, []byte("{not json"), 0o600); wErr != nil {
		t.Fatalf("write corrupt profiles: %v", wErr)
	}

	f, err := factory.New("0.0.0-test")
	if err == nil {
		t.Fatal("expected an error for a corrupt profiles.json, got nil")
	}
	code, ok := errcode.CodeFor(err)
	if !ok || code != errcode.CodeValidationFailed {
		t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeValidationFailed)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error must name the broken file %q, got %q", path, err.Error())
	}
	if f == nil {
		t.Fatal("New must still return a factory so the error can be rendered")
	}
	if f.Endpoint != "" {
		t.Fatalf("Endpoint = %q, want empty: a corrupt config must never silently resolve to production",
			f.Endpoint)
	}
}
