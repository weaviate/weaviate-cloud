package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

// tokenError carries the HTTP status and body of a non-2xx token-endpoint response.
type tokenError struct {
	status int
	body   string
}

const maxTokenResponseBytes = 1 << 20

func (e *tokenError) Error() string {
	return fmt.Sprintf("token exchange failed: HTTP %d: %s", e.status, e.body)
}

func exchangeCode(
	ctx context.Context,
	cfg config.AuthConfig,
	httpClient *http.Client,
	policy allowlist.Policy,
	code, redirectURI, verifier string,
) (*TokenResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", cfg.ClientID)
	form.Set("code_verifier", verifier)
	return requestToken(ctx, cfg, httpClient, policy, form)
}

// refreshTokens exchanges a refresh token for a fresh token set using the
// public-client grant (no client secret — the CLI is a public PKCE client).
func refreshTokens(
	ctx context.Context,
	cfg config.AuthConfig,
	httpClient *http.Client,
	policy allowlist.Policy,
	refreshToken string,
) (*TokenResult, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.ClientID)
	form.Set("refresh_token", refreshToken)
	return requestToken(ctx, cfg, httpClient, policy, form)
}

func requestToken(
	ctx context.Context,
	cfg config.AuthConfig,
	httpClient *http.Client,
	policy allowlist.Policy,
	form url.Values,
) (*TokenResult, error) {
	if err := policy.Check(tokenURL(cfg.BaseURL)); err != nil {
		return nil, errcode.New(errcode.CodeValidationFailed, fmt.Sprintf("refusing to send request: %v", err))
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURL(cfg.BaseURL),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errcode.NoResponse(errcode.PeerAuthServer, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if len(raw) > maxTokenResponseBytes {
		return nil, fmt.Errorf("token response exceeded %d bytes", maxTokenResponseBytes)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &tokenError{status: resp.StatusCode, body: strings.TrimSpace(string(raw))}
	}

	var tr TokenResult
	if decodeErr := json.Unmarshal(raw, &tr); decodeErr != nil {
		return nil, fmt.Errorf("decode token response: %w", decodeErr)
	}
	return &tr, nil
}
