//go:build !wcloud_dev

package allowlist

// devLoopbackEnabled is a compile-time constant, not a runtime flag, so an
// attacker who controls an env var or CLI flag cannot enable loopback.
const devLoopbackEnabled = false
