package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type TokenResult struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func runLogin(
	ctx context.Context,
	cfg config.AuthConfig,
	httpClient *http.Client,
	policy allowlist.Policy,
	streams *iostreams.IOStreams,
	opts LoginOptions,
) (*TokenResult, error) {
	if err := policy.Check(authorizeURL(cfg.BaseURL)); err != nil {
		return nil, errcode.New(
			errcode.CodeValidationFailed,
			fmt.Sprintf("refusing to open the sign-in URL: %v", err),
		)
	}
	pkce, err := newPKCE()
	if err != nil {
		return nil, err
	}
	state, err := randomState()
	if err != nil {
		return nil, err
	}

	deadline := opts.Now().Add(opts.Timeout)
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	loop, err := startLoopback(waitCtx, state)
	if err != nil {
		cancel()
		return nil, err
	}
	defer loop.close()
	defer cancel()

	w := streams.Err
	authURL := buildAuthorizeURL(cfg, loop.redirectURI, state, pkce.Challenge)
	printSignInURL(w, authURL)
	if opts.NoLaunchBrowser {
		fmt.Fprintln(w, "Not opening a browser (--no-launch-browser).")
	} else {
		fmt.Fprintln(w, "Opening it in your default browser now.")
		if openErr := openBrowser(ctx, authURL); openErr != nil {
			fmt.Fprintf(w, "(warning) the browser opener exited with an error: %v\n", openErr)
			fmt.Fprintln(w, "If a browser did not appear, open the URL yourself:")
			printSignInURL(w, authURL)
		}
	}

	humanWatching := streams.IsStderrTTY() && output.HumanReaderLikely()
	renderer, interval := newWaitRenderer(w, authURL, deadline, opts.Now, humanWatching)
	code, err := loop.awaitCode(waitCtx, renderer, interval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, loginTimeoutError(opts.Timeout)
		}
		return nil, err
	}
	tr, err := exchangeCode(ctx, cfg, httpClient, policy, code, loop.redirectURI, pkce.Verifier)
	if err != nil {
		return nil, exchangeFailure(err)
	}
	return tr, nil
}

// exchangeFailure reclassifies a rejected token exchange as auth_required, using the
// same rejected-grant-vs-transient predicate provider.go's RequireToken already applies
// to a token refresh (4xx except 429 = rejected grant, re-login; 429/5xx/network are
// transient — see provider.go:130-136 and its locked tests in require_test.go). A 429 or
// 5xx from the token endpoint is not "you are not authenticated," it is "the server is
// rate-limiting or momentarily down," so it is left unclassified (falls through to
// internal_error) rather than sent down the auth_required remedy that will not fix it.
// A non-HTTP failure (decode error, oversized body) is left as-is too; those are not
// answers from the token endpoint.
func exchangeFailure(err error) error {
	var te *tokenError
	if errors.As(err, &te) && te.status >= 400 && te.status < 500 && te.status != http.StatusTooManyRequests {
		return &errcode.Error{Code: errcode.CodeAuthRequired, Message: err.Error(), Cause: err}
	}
	return err
}

func buildAuthorizeURL(cfg config.AuthConfig, redirectURI, state, challenge string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", defaultScope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	return authorizeURL(cfg.BaseURL) + "?" + q.Encode()
}

func loginTimeoutError(waited time.Duration) error {
	return &errcode.Error{
		Code: errcode.CodeAuthRequired,
		Message: fmt.Sprintf(
			"timed out after %s waiting for the browser sign-in to complete. "+
				"The sign-in URL from this run has expired with it, so opening it now will not work. "+
				"Run 'wcloud auth login' again for a fresh URL: add a longer --timeout "+
				"(for example --timeout 10m) if the sign-in needs more time, or --no-launch-browser "+
				"if no browser appeared and you want to open the URL yourself in a browser on this machine",
			waited,
		),
		Details: map[string]any{
			"waited": waited.String(),
		},
	}
}
