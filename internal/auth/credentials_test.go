//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/config"
)

func TestSaveLoadCredentialsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	tr := &TokenResult{
		AccessToken:  encodeFakeJWT(map[string]any{"sub": "u1"}),
		IDToken:      encodeFakeJWT(map[string]any{"email": "a@b.io"}),
		RefreshToken: "rt",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Scope:        "openid",
	}
	path, err := saveCredentials("dev", tr)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !strings.HasSuffix(path, "credentials.dev.json") {
		t.Fatalf("path = %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o", info.Mode().Perm())
	}

	got, err := loadCredentials("dev")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AccessToken != tr.AccessToken || got.Scope != "openid" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Email() != "a@b.io" {
		t.Fatalf("email = %q, want a@b.io", got.Email())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw credentials: %v", err)
	}
	if strings.Contains(string(raw), `"claims"`) {
		t.Fatalf("credentials file must not persist decoded JWT claims:\n%s", raw)
	}
}

func TestLoadCredentialsMissingReturnsErrNotExist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	_, err := loadCredentials(config.DefaultProfileName)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestSaveCredentialsRejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := saveCredentials("dev", nil); err == nil {
		t.Fatal("expected error on nil TokenResult")
	}
	if _, err := saveCredentials("dev", &TokenResult{}); err == nil {
		t.Fatal("expected error on empty TokenResult")
	}
}

func TestDecodeJWTClaimsRejectsNonJWT(t *testing.T) {
	t.Parallel()
	if _, err := decodeJWTClaims("not.even"); err == nil {
		t.Fatal("expected error on too few parts")
	}
	if _, err := decodeJWTClaims("a.b.c.d"); err == nil {
		t.Fatal("expected error on too many parts")
	}
}

func TestSaveCredentialsRetightensLoosenedFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	tr := &TokenResult{AccessToken: "AT1", TokenType: "Bearer"}
	path, err := saveCredentials("dev", tr)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if chmodErr := os.Chmod(path, 0o644); chmodErr != nil {
		t.Fatalf("pre-loosen: %v", chmodErr)
	}

	if _, saveErr := saveCredentials("dev", &TokenResult{AccessToken: "AT2", TokenType: "Bearer"}); saveErr != nil {
		t.Fatalf("second save: %v", saveErr)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm = %o, want 0600 (re-tightened)", mode)
	}
}

func TestSaveCredentialsRetightensLoosenedDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	tr := &TokenResult{AccessToken: "AT1", TokenType: "Bearer"}
	path, err := saveCredentials("dev", tr)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	dir := parentDir(path)
	if chmodErr := os.Chmod(dir, 0o755); chmodErr != nil {
		t.Fatalf("pre-loosen dir: %v", chmodErr)
	}

	if _, saveErr := saveCredentials("dev", &TokenResult{AccessToken: "AT2", TokenType: "Bearer"}); saveErr != nil {
		t.Fatalf("second save: %v", saveErr)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("dir perm = %o, want 0700 (re-tightened)", mode)
	}
}

func TestLoadCredentialsRetightensPermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	tr := &TokenResult{AccessToken: "AT1", TokenType: "Bearer"}
	path, err := saveCredentials("dev", tr)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if chmodErr := os.Chmod(path, 0o644); chmodErr != nil {
		t.Fatalf("pre-loosen: %v", chmodErr)
	}

	if _, loadErr := loadCredentials("dev"); loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm after load = %o, want 0600 (re-tightened)", mode)
	}
}

func encodeFakeJWT(payload map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body, _ := json.Marshal(payload)
	encBody := base64.RawURLEncoding.EncodeToString(body)
	return header + "." + encBody + "." + "fakesig"
}
