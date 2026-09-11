package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/config"
)

func TestValidateProfileName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"reserved default", "default", true},
		{"uppercase rejected", "Dev", true},
		{"starts with dash rejected", "-dev", true},
		{"starts with digit ok", "1dev", false},
		{"kebab case ok", "dev-staging", false},
		{"single char ok", "d", false},
		{"underscore rejected", "dev_staging", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := config.ValidateProfileName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateProfileName(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestValidateProfile(t *testing.T) {
	t.Parallel()
	validAuth := &config.AuthConfig{
		BaseURL:  "https://auth.example.com",
		ClientID: "C1",
	}
	tests := []struct {
		name    string
		input   config.Profile
		wantErr bool
	}{
		{"empty profile ok", config.Profile{}, false},
		{"endpoint only ok", config.Profile{Endpoint: "https://api.example.com"}, false},
		{"endpoint missing scheme", config.Profile{Endpoint: "api.example.com"}, true},
		{"endpoint ftp rejected", config.Profile{Endpoint: "ftp://api.example.com"}, true},
		{"endpoint missing host", config.Profile{Endpoint: "https://"}, true},
		{"full auth ok", config.Profile{Auth: validAuth}, false},
		{
			"auth missing client",
			config.Profile{Auth: &config.AuthConfig{BaseURL: "https://x"}},
			true,
		},
		{
			"auth missing base url",
			config.Profile{Auth: &config.AuthConfig{ClientID: "C"}},
			true,
		},
		{"empty auth struct rejected", config.Profile{Auth: &config.AuthConfig{}}, true},
		{"endpoint and full auth ok", config.Profile{Endpoint: "https://api.example.com", Auth: validAuth}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := config.ValidateProfile(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateProfile(%+v) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestActiveProfileFallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   config.Profiles
		want string
	}{
		{"empty active means default", config.Profiles{Active: "", Profiles: map[string]config.Profile{}}, "default"},
		{
			"unknown active falls back to default",
			config.Profiles{Active: "ghost", Profiles: map[string]config.Profile{"dev": {}}},
			"default",
		},
		{
			"known active wins",
			config.Profiles{Active: "dev", Profiles: map[string]config.Profile{"dev": {}}},
			"dev",
		},
		{"explicit default ok", config.Profiles{Active: "default"}, "default"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := config.ActiveProfile(tc.in)
			if got != tc.want {
				t.Fatalf("ActiveProfile = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveWithPrecedence(t *testing.T) {
	for _, key := range []string{
		config.EndpointEnvVar,
		config.AuthBaseURLEnvVar,
		config.AuthClientIDEnvVar,
	} {
		t.Setenv(key, "")
	}

	//nolint:paralleltest // parent uses t.Setenv
	t.Run("no profile, no env returns defaults", func(t *testing.T) {
		cfg := config.ResolveWith(config.Profiles{Active: config.DefaultProfileName})
		if cfg.Endpoint != config.ProductionEndpoint {
			t.Fatalf("endpoint = %q, want %q", cfg.Endpoint, config.ProductionEndpoint)
		}
		if cfg.Auth.BaseURL == "" || cfg.Auth.ClientID == "" {
			t.Fatalf("auth should be populated from built-in defaults: %+v", cfg.Auth)
		}
	})

	//nolint:paralleltest // parent uses t.Setenv
	t.Run("profile values applied when active", func(t *testing.T) {
		p := config.Profiles{
			Active: "dev",
			Profiles: map[string]config.Profile{
				"dev": {
					Endpoint: "https://api.dev.example.com",
					Auth: &config.AuthConfig{
						BaseURL:  "https://auth.dev.example.com",
						ClientID: "Cdev",
					},
				},
			},
		}
		cfg := config.ResolveWith(p)
		if cfg.Endpoint != "https://api.dev.example.com" {
			t.Fatalf("endpoint = %q", cfg.Endpoint)
		}
		if cfg.Auth.ClientID != "Cdev" {
			t.Fatalf("auth.client_id = %q", cfg.Auth.ClientID)
		}
	})

	t.Run("env overrides profile per-field", func(t *testing.T) {
		t.Setenv(config.AuthClientIDEnvVar, "Cenv")
		p := config.Profiles{
			Active: "dev",
			Profiles: map[string]config.Profile{
				"dev": {Auth: &config.AuthConfig{BaseURL: "https://x", ClientID: "Cdev"}},
			},
		}
		cfg := config.ResolveWith(p)
		if cfg.Auth.ClientID != "Cenv" {
			t.Fatalf("auth.client_id = %q, want Cenv", cfg.Auth.ClientID)
		}
		if cfg.Auth.BaseURL != "https://x" {
			t.Fatalf("auth.base_url = %q, want https://x (profile)", cfg.Auth.BaseURL)
		}
	})

	//nolint:paralleltest // parent uses t.Setenv
	t.Run("default profile ignores profile map", func(t *testing.T) {
		p := config.Profiles{
			Active: config.DefaultProfileName,
			Profiles: map[string]config.Profile{
				"dev": {Endpoint: "https://x"},
			},
		}
		cfg := config.ResolveWith(p)
		if cfg.Endpoint != config.ProductionEndpoint {
			t.Fatalf("endpoint = %q, want production default", cfg.Endpoint)
		}
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	in := config.Profiles{
		Active: "dev",
		Profiles: map[string]config.Profile{
			"dev": {
				Endpoint: "https://api.dev.example.com",
				Auth:     &config.AuthConfig{BaseURL: "https://a", ClientID: "C"},
			},
		},
	}
	if err := config.SaveProfiles(in); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	got, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if got.Active != in.Active {
		t.Fatalf("active = %q, want %q", got.Active, in.Active)
	}
	if got.Profiles["dev"].Endpoint != in.Profiles["dev"].Endpoint {
		t.Fatalf("endpoint = %q, want %q",
			got.Profiles["dev"].Endpoint, in.Profiles["dev"].Endpoint)
	}
	if got.Profiles["dev"].Auth == nil ||
		got.Profiles["dev"].Auth.ClientID != "C" {
		t.Fatalf("auth round-trip failed: got %+v", got.Profiles["dev"].Auth)
	}
}

func TestLoadProfilesMissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	p, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles on missing file should not error: %v", err)
	}
	if p.Active != config.DefaultProfileName {
		t.Fatalf("active = %q, want default", p.Active)
	}
	if len(p.Profiles) != 0 {
		t.Fatalf("profiles = %v, want empty", p.Profiles)
	}
}

func TestLoadProfilesMalformedJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	// Save once to ensure the file exists in the platform-correct location,
	// then overwrite that path with invalid JSON.
	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	configDir, _ := os.UserConfigDir()
	path := filepath.Join(configDir, "wcloud", config.ProfilesFileName)
	if writeErr := os.WriteFile(path, []byte("{not json"), 0o600); writeErr != nil {
		t.Fatalf("overwrite: %v", writeErr)
	}

	if _, loadErr := config.LoadProfiles(); loadErr == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestSaveProfilesRetightensLoosenedFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	configDir, _ := os.UserConfigDir()
	path := filepath.Join(configDir, "wcloud", config.ProfilesFileName)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("pre-loosen: %v", err)
	}

	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("second SaveProfiles: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm = %o, want 0600 (re-tightened)", mode)
	}
}

func TestSaveProfilesRetightensLoosenedDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	configDir, _ := os.UserConfigDir()
	dir := filepath.Join(configDir, "wcloud")
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("pre-loosen dir: %v", err)
	}

	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("second SaveProfiles: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("dir perm = %o, want 0700 (re-tightened)", mode)
	}
}

func TestLoadProfilesRetightensPermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	configDir, _ := os.UserConfigDir()
	path := filepath.Join(configDir, "wcloud", config.ProfilesFileName)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("pre-loosen: %v", err)
	}

	if _, err := config.LoadProfiles(); err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm after load = %o, want 0600 (re-tightened)", mode)
	}
}

func TestSaveProfilesPermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := config.SaveProfiles(config.Profiles{Active: "default"}); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	configDir, _ := os.UserConfigDir()
	path := filepath.Join(configDir, "wcloud", config.ProfilesFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file perm = %o, want 0600", mode)
	}

	// also ensure the JSON is well-formed
	raw, _ := os.ReadFile(path)
	var anyJSON map[string]any
	if jsonErr := json.Unmarshal(raw, &anyJSON); jsonErr != nil {
		t.Fatalf("written file is not valid JSON: %v", jsonErr)
	}
}
