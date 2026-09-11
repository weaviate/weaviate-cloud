//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package iostreams

import (
	"os"
	"testing"
)

// TestIsTerminalDiscriminatesRealDescriptors drives isTerminal with genuine file
// descriptors rather than a stub, so a build that always answers "yes" fails
// here instead of silently emitting human text into a machine consumer's pipe.
func TestIsTerminalDiscriminatesRealDescriptors(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })

	regular, err := os.CreateTemp(t.TempDir(), "not-a-tty")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { _ = regular.Close() })

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })

	cases := []struct {
		name string
		f    *os.File
	}{
		{"nil descriptor", nil},
		{"os.DevNull is a character device but not a terminal", devNull},
		{"a regular file", regular},
		{"the read end of a pipe", pr},
		{"the write end of a pipe", pw},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if isTerminal(tc.f) {
				t.Errorf("isTerminal(%s) = true, want false", tc.name)
			}
		})
	}
}
