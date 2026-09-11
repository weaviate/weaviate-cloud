//go:build !darwin && !linux

package iostreams

import "os"

// isTerminal falls back to checking for a character device on platforms other
// than Darwin and Linux, where we do not have a ioctl-based terminal check.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
