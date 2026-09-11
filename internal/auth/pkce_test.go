//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewPKCEChallengeMatchesVerifier(t *testing.T) {
	t.Parallel()
	p, err := newPKCE()
	if err != nil {
		t.Fatalf("newPKCE: %v", err)
	}
	if p.Verifier == "" || p.Challenge == "" {
		t.Fatal("verifier and challenge must be non-empty")
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Fatalf("challenge mismatch:\n  got:  %q\n  want: %q", p.Challenge, want)
	}
}

func TestNewPKCEVerifierUsesURLSafeAlphabet(t *testing.T) {
	t.Parallel()
	p, err := newPKCE()
	if err != nil {
		t.Fatalf("newPKCE: %v", err)
	}
	const allowed = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, r := range p.Verifier {
		if !strings.ContainsRune(allowed, r) {
			t.Fatalf("verifier contains non-PKCE rune %q in %q", r, p.Verifier)
		}
	}
}

func TestRandomStateIsURLSafe(t *testing.T) {
	t.Parallel()
	s, err := randomState()
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	if _, decodeErr := base64.RawURLEncoding.DecodeString(s); decodeErr != nil {
		t.Fatalf("state is not url-safe base64: %v", decodeErr)
	}
}
