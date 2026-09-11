package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/config"
)

const jwtPartCount = 3

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	IDToken      string    `json:"id_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitzero"`
	Scope        string    `json:"scope,omitempty"`
	SavedAt      time.Time `json:"saved_at"`
}

func (c *Credentials) Email() string {
	claims, err := decodeJWTClaims(c.IDToken)
	if err != nil {
		return ""
	}
	email, _ := claims["email"].(string)
	return email
}

func saveCredentials(profile string, tr *TokenResult) (string, error) {
	if tr == nil || tr.AccessToken == "" {
		return "", errors.New("nothing to save: empty token result")
	}

	creds := Credentials{
		AccessToken:  tr.AccessToken,
		IDToken:      tr.IDToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Scope:        tr.Scope,
		SavedAt:      time.Now().UTC(),
	}
	if tr.ExpiresIn > 0 {
		creds.ExpiresAt = creds.SavedAt.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	path, err := config.CredentialsPath(profile)
	if err != nil {
		return "", err
	}
	if mkErr := config.EnsureDir(filepath.Dir(path), config.DirPerm); mkErr != nil {
		return "", fmt.Errorf("create config dir: %w", mkErr)
	}
	//nolint:gosec // intentional: persisting auth tokens on disk
	buf, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode credentials: %w", err)
	}
	if writeErr := config.WriteFileAtomic(path, buf, config.FilePerm); writeErr != nil {
		return "", fmt.Errorf("write credentials: %w", writeErr)
	}
	return path, nil
}

func loadCredentials(profile string) (*Credentials, error) {
	path, err := config.CredentialsPath(profile)
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Re-tighten a file any external actor loosened (backup restore, manual
	// chmod, old CLI version); a chmod failure must not block a valid read.
	_ = os.Chmod(path, config.FilePerm)
	var c Credentials
	if decodeErr := json.Unmarshal(buf, &c); decodeErr != nil {
		return nil, fmt.Errorf("%w: %w", errCorruptCredentials, decodeErr)
	}
	return &c, nil
}

// errCorruptCredentials marks a credentials file that exists and was read
// but failed to decode, distinct from [os.ErrNotExist]. RequireToken maps
// both to the same auth_required code (frozen contract) with different
// wording.
var errCorruptCredentials = errors.New("credentials file is corrupt")

// decodeJWTClaims unpacks a JWT payload without verifying the signature —
// safe here because we just received the token over TLS from Descope.
func decodeJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != jwtPartCount {
		return nil, errors.New("not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims map[string]any
	if parseErr := json.Unmarshal(raw, &claims); parseErr != nil {
		return nil, fmt.Errorf("parse JWT claims: %w", parseErr)
	}
	return claims, nil
}
