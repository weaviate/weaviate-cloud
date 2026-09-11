package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// DefaultProfileName is the implicit, built-in profile. It is never stored
	// on disk; absence of any active profile resolves to this name and to the
	// compiled-in production defaults.
	DefaultProfileName = "default"

	ProfilesFileName = "profiles.json"
)

var profileNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Profile struct {
	Endpoint string      `json:"endpoint,omitempty"`
	Auth     *AuthConfig `json:"auth,omitempty"`
}

type Profiles struct {
	Active   string             `json:"active"`
	Profiles map[string]Profile `json:"profiles"`
}

type Config struct {
	Endpoint string
	Auth     AuthConfig
}

// LoadProfiles reads profiles.json from the user config dir. A missing file
// is not an error: callers receive an empty Profiles with Active=default.
func LoadProfiles() (Profiles, error) {
	path, err := profilesPath()
	if err != nil {
		return Profiles{}, err
	}
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Profiles{Active: DefaultProfileName, Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return Profiles{}, fmt.Errorf("read profiles: %w", err)
	}
	// Re-tighten a file any external actor loosened; a chmod failure must
	// not block a valid read.
	_ = os.Chmod(path, FilePerm)
	var p Profiles
	if jsonErr := json.Unmarshal(buf, &p); jsonErr != nil {
		return Profiles{}, fmt.Errorf("decode profiles: %w", jsonErr)
	}
	if p.Profiles == nil {
		p.Profiles = map[string]Profile{}
	}
	if p.Active == "" {
		p.Active = DefaultProfileName
	}
	return p, nil
}

func SaveProfiles(p Profiles) error {
	path, err := profilesPath()
	if err != nil {
		return err
	}
	if mkErr := EnsureDir(filepath.Dir(path), DirPerm); mkErr != nil {
		return fmt.Errorf("create config dir: %w", mkErr)
	}
	buf, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profiles: %w", err)
	}
	if writeErr := WriteFileAtomic(path, buf, FilePerm); writeErr != nil {
		return fmt.Errorf("write profiles: %w", writeErr)
	}
	return nil
}

// ActiveProfile returns the active profile name, falling back to default
// when the stored active name does not exist. Never errors.
func ActiveProfile(p Profiles) string {
	if p.Active == DefaultProfileName || p.Active == "" {
		return DefaultProfileName
	}
	if _, ok := p.Profiles[p.Active]; ok {
		return p.Active
	}
	return DefaultProfileName
}

// ResolveWith composes the active configuration with precedence
// env var > active profile value > built-in default (per-field).
func ResolveWith(p Profiles) Config {
	cfg := Config{
		Endpoint: ProductionEndpoint,
		Auth:     defaultAuth(),
	}

	if active := ActiveProfile(p); active != DefaultProfileName {
		prof := p.Profiles[active]
		if prof.Endpoint != "" {
			cfg.Endpoint = prof.Endpoint
		}
		if prof.Auth != nil {
			cfg.Auth = *prof.Auth
		}
	}

	if v := lookupEnv(EndpointEnvVar); v != "" {
		cfg.Endpoint = v
	}
	if v := lookupEnv(AuthBaseURLEnvVar); v != "" {
		cfg.Auth.BaseURL = v
	}
	if v := lookupEnv(AuthClientIDEnvVar); v != "" {
		cfg.Auth.ClientID = v
	}

	return cfg
}

// ValidateProfileName rejects empty names, the reserved "default", and
// anything outside the kebab-case alphanumeric set.
func ValidateProfileName(name string) error {
	if name == "" {
		return errors.New("profile name is required")
	}
	if name == DefaultProfileName {
		return errors.New("'default' is a reserved profile name")
	}
	if !profileNameRE.MatchString(name) {
		return fmt.Errorf("profile name %q must match %s", name, profileNameRE.String())
	}
	return nil
}

// ValidateProfile enforces: endpoint (if set) is http(s) with a host; auth
// is all-or-nothing — a present Auth pointer requires base_url and client_id
// both populated.
func ValidateProfile(p Profile) error {
	if p.Endpoint != "" {
		if err := validateHTTPURL(p.Endpoint); err != nil {
			return fmt.Errorf("endpoint: %w", err)
		}
	}
	if p.Auth == nil {
		return nil
	}
	if p.Auth.BaseURL == "" && p.Auth.ClientID == "" {
		return errors.New("auth: at least one field is required when auth is present")
	}
	if err := validateHTTPURL(p.Auth.BaseURL); err != nil {
		return fmt.Errorf("auth.base_url: %w", err)
	}
	if strings.TrimSpace(p.Auth.ClientID) == "" {
		return errors.New("auth.client_id: required")
	}
	return nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	return nil
}

func profilesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "wcloud", ProfilesFileName), nil
}
