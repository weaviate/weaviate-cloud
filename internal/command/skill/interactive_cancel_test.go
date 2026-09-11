//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package skill

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// blockingReader never returns, standing in for a terminal with no one typing.
type blockingReader struct{ release chan struct{} }

func (b *blockingReader) Read(_ []byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

func TestInteractiveSelectAbortsOnContextCancel(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	ctx, cancel := context.WithCancel(context.Background())
	var errOut bytes.Buffer

	done := make(chan error, 1)
	go func() {
		_, err := interactiveSelect(ctx, &blockingReader{release: release}, &errOut, knownHarnesses)
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want it to wrap context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("interactiveSelect ignored the cancelled context and is still blocked on stdin")
	}
}
