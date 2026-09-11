package auth

const (
	authorizeAppsPath = "/oauth2/v1/apps/authorize"
	revokeAppsPath    = "/oauth2/v1/apps/revoke"

	defaultScope = "openid profile email cluster:create cluster:read region:read account:read"

	loopbackHost = "127.0.0.1"
	loopbackPath = "/callback"
)

//nolint:gosec // URL path, not a credential
const tokenAppsPath = "/oauth2/v1/apps/token"

// loopbackPorts are tried in order; the first port that successfully binds
// wins. All ports must be pre-registered as approvedCallbackUrls on this
// CLI's Descope Inbound App (Descope does exact-match on redirect_uri).
//
// Chosen from IANA's dynamic/ephemeral range (49152–65535) where collisions
// with conventional dev servers (3000, 5000, 8080, etc.) are negligible.
//
//nolint:gochecknoglobals // OAuth callback ports must match the static list registered on the Descope Inbound App
var loopbackPorts = []int{53682, 53683, 53684, 53685}

func authorizeURL(baseURL string) string { return baseURL + authorizeAppsPath }
func tokenURL(baseURL string) string     { return baseURL + tokenAppsPath }
func revokeURL(baseURL string) string    { return baseURL + revokeAppsPath }
