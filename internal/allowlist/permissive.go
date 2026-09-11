package allowlist

// NewPermissiveForTesting exists so tests can build a policy for httptest
// servers on unpredictable loopback ports. Calling it from a non-test file
// trips the "credential-allowlist-test-only" forbidigo rule in .golangci.yml.
func NewPermissiveForTesting() Policy { return Policy{mode: modePermissive} }
