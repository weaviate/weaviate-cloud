package allowlist

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const allowedRoot = "weaviate.cloud"

type policyMode int

const (
	modeUnconfigured policyMode = iota
	modeStrict
	modePermissive
)

// Policy decides whether a credential may be sent to a URL. The zero value
// is modeUnconfigured and fails closed — there is no way to construct a
// permissive Policy from outside this package; see NewPermissiveForTesting's
// doc comment for the one, deliberately narrow, test-only exception.
type Policy struct {
	mode policyMode
}

// Production is the strict policy every production constructor
// (api.NewClient, auth.NewProvider) uses unless a test explicitly
// overrides it. It permits only https to weaviate.cloud or a subdomain of
// it, label-aware, plus (only in a developer build — see loopback_dev.go)
// http to a loopback IP literal.
func Production() Policy { return Policy{mode: modeStrict} }

func (p Policy) Check(rawURL string) error {
	switch p.mode {
	case modePermissive:
		return nil
	case modeStrict:
		return checkStrict(rawURL)
	case modeUnconfigured:
		return errors.New("credential-destination policy is not configured")
	default:
		return errors.New("credential-destination policy is not configured")
	}
}

func checkStrict(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid destination URL: %w", err)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")

	if devLoopbackEnabled && isLoopbackHost(host) {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("destination scheme must be https, got %q", u.Scheme)
	}
	if !isPermittedHost(host) {
		return fmt.Errorf("destination host %q is not a permitted credential destination", host)
	}
	return nil
}

func isPermittedHost(host string) bool {
	return host == allowedRoot || strings.HasSuffix(host, "."+allowedRoot)
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
