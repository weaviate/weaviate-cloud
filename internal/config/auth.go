package config

import "os"

const (
	AuthBaseURLEnvVar  = "WCLOUD_AUTH_BASE_URL"
	AuthClientIDEnvVar = "WCLOUD_AUTH_CLIENT_ID"
)

// Production auth-provider defaults. The CLI ships preconfigured so
// `wcloud auth login` works out of the box on a clean machine — no env
// vars or profile required. Environment names (dev/prod) are deliberately
// never surfaced to the user; this constant is "the" auth provider as far
// as the public binary is concerned.
const (
	defaultAuthBaseURL  = "https://auth.weaviate.cloud"
	defaultAuthClientID = "UGV1YzEyeTAyVUEwZUFFRDFkcVNqRTVIdEdVcnBCc3g6VFBBM0VkOEVPZ2tJa2NqUjJLak1WUEVWR00zOE4y"
)

type AuthConfig struct {
	BaseURL  string `json:"base_url,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

func defaultAuth() AuthConfig {
	return AuthConfig{
		BaseURL:  defaultAuthBaseURL,
		ClientID: defaultAuthClientID,
	}
}

func lookupEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return ""
	}
	return v
}
