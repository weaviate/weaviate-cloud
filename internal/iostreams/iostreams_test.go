package iostreams_test

import (
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func TestSetTTY(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()

	if ios.IsStdinTTY() {
		t.Fatal("Test() should return non-TTY stdin")
	}
	ios.SetStdinTTY(true)
	if !ios.IsStdinTTY() {
		t.Fatal("SetStdinTTY(true) did not flip IsStdinTTY")
	}

	if ios.IsStdoutTTY() {
		t.Fatal("Test() should return non-TTY stdout")
	}
	ios.SetStdoutTTY(true)
	if !ios.IsStdoutTTY() {
		t.Fatal("SetStdoutTTY(true) did not flip IsStdoutTTY")
	}

	if ios.IsStderrTTY() {
		t.Fatal("Test() should return non-TTY stderr")
	}
	ios.SetStderrTTY(true)
	if !ios.IsStderrTTY() {
		t.Fatal("SetStderrTTY(true) did not flip IsStderrTTY")
	}
}
