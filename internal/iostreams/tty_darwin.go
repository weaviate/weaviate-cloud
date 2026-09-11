//go:build darwin

package iostreams

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal returns true only when f is a real terminal.
// Uses TIOCGETA to distinguish terminals from other character devices (/dev/null etc.).
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCGETA,
		uintptr(unsafe.Pointer(&termios)),
	)
	return errno == 0
}
