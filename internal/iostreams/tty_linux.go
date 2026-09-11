//go:build linux

package iostreams

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal returns true only when f is a real terminal.
// Uses TCGETS to distinguish terminals from other character devices (/dev/null etc.).
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TCGETS,
		uintptr(unsafe.Pointer(&termios)),
	)
	return errno == 0
}
