package allowlist_test

import (
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
)

func TestProductionCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{"production API host", "https://api-cloud.weaviate.cloud/v1/whoami", false},
		{"production auth host", "https://auth.weaviate.cloud/oauth2/v1/apps/token", false},
		{"bare allowed root", "https://weaviate.cloud/", false},
		{"multi-level subdomain", "https://a.b.c.weaviate.cloud/", false},
		{"uppercase root", "https://WEAVIATE.CLOUD/", false},
		{"mixed-case subdomain", "https://Api-Cloud.Weaviate.Cloud/", false},
		{"trailing dot on root", "https://weaviate.cloud./", false},
		{"non-default port on allowed host", "https://api-cloud.weaviate.cloud:8443/", false},
		{"benign userinfo, allowed host", "https://attacker@api-cloud.weaviate.cloud/", false},
		{"label-aware: evil prefix, not a subdomain", "https://evil-weaviate.cloud/", true},
		{"label-aware: allowed root as a prefix of an evil domain", "https://weaviate.cloud.evil.com/", true},
		{"trailing dot does not rescue an evil suffix", "https://weaviate.cloud.evil.com./", true},
		{"http on an otherwise-allowed host", "http://api-cloud.weaviate.cloud/", true},
		{"https on a disallowed host", "https://evil.com/", true},
		{"userinfo trick: real host is the attacker's", "https://weaviate.cloud@evil.com/", true},
		{"non-loopback IP literal, never permitted", "https://203.0.113.10/", true},
		{"IDN/punycode-looking host, no legitimate match", "https://xn--weaviate-cloud-evil.example/", true},
		{"malformed URL", "://bad", true},
		{"empty host", "https:///path", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := allowlist.Production().Check(tc.rawURL)
			if (err != nil) != tc.wantErr {
				t.Errorf("Check(%q) error = %v, wantErr %v", tc.rawURL, err, tc.wantErr)
			}
		})
	}
}

func TestPolicyZeroValueFailsClosed(t *testing.T) {
	t.Parallel()
	var p allowlist.Policy
	if err := p.Check("https://api-cloud.weaviate.cloud/"); err == nil {
		t.Fatal("zero-value Policy must reject even an otherwise-allowed host")
	}
}

func TestNewPermissiveForTestingAllowsEverything(t *testing.T) {
	t.Parallel()
	p := allowlist.NewPermissiveForTesting()
	for _, u := range []string{"http://127.0.0.1:9999/", "https://evil.com/", "not a url"} {
		if err := p.Check(u); err != nil {
			t.Errorf("Check(%q) = %v, want nil (permissive policy)", u, err)
		}
	}
}
