package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

// Descope's inbound-app revoke endpoint rejects the client id alone — it must
// be authenticated by the user's access token as a Bearer.
func revokeRefreshToken(
	ctx context.Context,
	cfg config.AuthConfig,
	httpClient *http.Client,
	policy allowlist.Policy,
	accessToken, refreshToken string,
) error {
	if err := policy.Check(revokeURL(cfg.BaseURL)); err != nil {
		return errcode.New(errcode.CodeValidationFailed, fmt.Sprintf("refusing to send request: %v", err))
	}
	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")
	form.Set("client_id", cfg.ClientID)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		revokeURL(cfg.BaseURL),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("build revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("revoke failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
