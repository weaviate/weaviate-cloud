package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

const authHTTPTimeout = 30 * time.Second

// Provider is the auth surface commands consume through the Factory.
type Provider struct {
	cfg     config.AuthConfig
	profile string
	streams *iostreams.IOStreams
	http    *http.Client
	policy  allowlist.Policy
}

const maxAuthRedirects = 3

func NewProvider(
	cfg config.AuthConfig,
	profile string,
	streams *iostreams.IOStreams,
	httpClient *http.Client,
) *Provider {
	policy := allowlist.Production()
	if httpClient == nil {
		httpClient = &http.Client{Timeout: authHTTPTimeout}
	}
	// WHY: Go replays a POST body on 307/308, so an unchecked hop off the token endpoint carries the refresh token or the code+verifier to the new origin.
	if httpClient.CheckRedirect == nil {
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxAuthRedirects {
				return fmt.Errorf("stopped after %d redirects", maxAuthRedirects)
			}
			if err := policy.Check(req.URL.String()); err != nil {
				return fmt.Errorf("refusing to follow redirect: %w", err)
			}
			return nil
		}
	}
	if profile == "" {
		profile = config.DefaultProfileName
	}
	return &Provider{cfg: cfg, profile: profile, streams: streams, http: httpClient, policy: policy}
}

const defaultLoginTimeout = 5 * time.Minute

// LoginOptions carries the auth login command's own flags into the flow.
type LoginOptions struct {
	Timeout         time.Duration
	NoLaunchBrowser bool
	Now             func() time.Time
}

func (p *Provider) Login(ctx context.Context, opts LoginOptions) (*TokenResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = defaultLoginTimeout
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return runLogin(ctx, p.cfg, p.http, p.policy, p.streams, opts)
}

func (p *Provider) Save(tr *TokenResult) (string, error) {
	return saveCredentials(p.profile, tr)
}

func (p *Provider) Load() (*Credentials, error) {
	return loadCredentials(p.profile)
}

// Logout best-effort revokes the refresh token server-side (a failure never
// blocks local sign-out), then removes the local credentials file.
func (p *Provider) Logout(ctx context.Context) error {
	if creds, err := p.Load(); err == nil && creds.AccessToken != "" && creds.RefreshToken != "" {
		revErr := revokeRefreshToken(ctx, p.cfg, p.http, p.policy, creds.AccessToken, creds.RefreshToken)
		if revErr != nil {
			fmt.Fprintf(p.streams.Err, "(warning) could not revoke refresh token: %v\n", revErr)
		}
	}
	return config.DeleteCredentials(p.profile)
}

// expirySkew refreshes slightly ahead of the deadline so a token never
// expires mid-request between the check and the API call that uses it.
const expirySkew = 60 * time.Second

// RequireToken returns a valid access token for the active profile, suitable
// for direct use as a bearer credential. When the stored token is within
// expirySkew of expiry and a refresh token is present, it transparently
// refreshes and persists the new token set. It returns a auth_required
// errcode (suitable for direct return from a command's RunE) when there is no
// usable session.
func (p *Provider) RequireToken(ctx context.Context) (string, error) {
	creds, err := p.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errNotSignedIn
		}
		if errors.Is(err, errCorruptCredentials) {
			return "", errCorruptCredentialsFile
		}
		return "", err
	}
	if creds.AccessToken == "" {
		return "", errNotSignedIn
	}
	if !tokenExpiring(creds) {
		return creds.AccessToken, nil
	}
	if creds.RefreshToken == "" {
		return "", errSessionExpired
	}

	refreshed, err := refreshTokens(ctx, p.cfg, p.http, p.policy, creds.RefreshToken)
	if err != nil {
		// 4xx except 429 = rejected grant (expired/revoked/consent withdrawn), re-login;
		// 429 and 5xx/network are transient.
		var te *tokenError
		if errors.As(err, &te) && te.status >= 400 && te.status < 500 && te.status != http.StatusTooManyRequests {
			return "", errSessionExpired
		}
		return "", fmt.Errorf("refresh access token: %w", err)
	}
	if refreshed.AccessToken == "" {
		return "", errSessionExpired
	}
	// Descope may rotate the refresh token or not; keep the existing one when
	// the response omits a replacement so the next refresh still works.
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = creds.RefreshToken
	}
	if _, saveErr := p.Save(refreshed); saveErr != nil {
		return "", fmt.Errorf("persist refreshed credentials: %w", saveErr)
	}
	return refreshed.AccessToken, nil
}

func tokenExpiring(creds *Credentials) bool {
	if creds.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(expirySkew).After(creds.ExpiresAt)
}

var (
	errNotSignedIn = errcode.New(
		errcode.CodeAuthRequired,
		"not signed in. Run 'wcloud auth login' to authenticate.",
	)
	errSessionExpired = errcode.New(
		errcode.CodeAuthRequired,
		"session expired. Run 'wcloud auth login' to re-authenticate.",
	)
	errCorruptCredentialsFile = errcode.New(
		errcode.CodeAuthRequired,
		"credentials file is corrupt. Run 'wcloud auth login' to sign in again.",
	)
)
