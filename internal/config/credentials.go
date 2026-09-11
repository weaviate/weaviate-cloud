package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CredentialsPath returns the absolute path of the on-disk credentials file
// for the given profile. The default profile uses credentials.json; named
// profiles use credentials.<name>.json.
func CredentialsPath(profileName string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	name := "credentials.json"
	if profileName != "" && profileName != DefaultProfileName {
		name = "credentials." + profileName + ".json"
	}
	return filepath.Join(dir, "wcloud", name), nil
}

// DeleteCredentials removes the credentials file for the given profile.
// A missing file is not an error.
func DeleteCredentials(profileName string) error {
	path, err := CredentialsPath(profileName)
	if err != nil {
		return err
	}
	if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return fmt.Errorf("remove credentials: %w", rmErr)
	}
	return nil
}
