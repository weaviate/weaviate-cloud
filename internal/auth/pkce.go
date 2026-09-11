package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const (
	pkceVerifierBytes = 32
	stateBytes        = 16
)

// PKCE holds an OAuth2 PKCE verifier + S256 challenge pair (RFC 7636).
type PKCE struct {
	Verifier  string
	Challenge string
}

func newPKCE() (*PKCE, error) {
	buf := make([]byte, pkceVerifierBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("pkce: read random: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return &PKCE{Verifier: verifier, Challenge: challenge}, nil
}

func randomState() (string, error) {
	buf := make([]byte, stateBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("state: read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
