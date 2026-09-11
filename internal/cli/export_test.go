package cli

// ResolvedVersionForTesting exposes resolvedVersion to black-box tests.
func ResolvedVersionForTesting() string {
	return resolvedVersion()
}
