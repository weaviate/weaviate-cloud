//go:build wcloud_dev

//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func TestRunLoginAcceptsALoopbackAuthorizeURLWithDevTag(t *testing.T) {
	t.Parallel()

	streams, _, _, _ := iostreams.Test()
	cfg := config.AuthConfig{BaseURL: "http://127.0.0.1:9", ClientID: "test-client"}

	_, err := runLogin(
		context.Background(),
		cfg,
		http.DefaultClient,
		allowlist.Production(),
		streams,
		LoginOptions{Timeout: 150 * time.Millisecond, NoLaunchBrowser: true, Now: time.Now},
	)

	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("err is %T, want *errcode.Error", err)
	}
	if ec.Code != errcode.CodeAuthRequired {
		t.Fatalf("code = %q, want the timeout %q — a developer build must still reach a loopback provider",
			ec.Code, errcode.CodeAuthRequired)
	}
}
