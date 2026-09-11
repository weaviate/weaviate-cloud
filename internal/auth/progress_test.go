//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWaitRendererLineModeIsParseCleanAndRepeatsTheURL(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0).UTC()
	deadline := t0.Add(5 * time.Minute)
	var buf bytes.Buffer

	r, interval := newWaitRenderer(&buf, "https://auth.example.com/authorize?x=1", deadline,
		func() time.Time { return t0.Add(30 * time.Second) }, false)
	if interval != waitLineInterval {
		t.Fatalf("interval = %v, want %v", interval, waitLineInterval)
	}
	r.onTick()
	r.onTick()
	r.onDone()

	out := buf.String()
	if strings.ContainsAny(out, "\x1b\r") {
		t.Fatalf("non-TTY output must contain no ESC or CR bytes, got %q", out)
	}
	if n := strings.Count(out, "https://auth.example.com/authorize?x=1"); n != 2 {
		t.Fatalf("URL appeared %d times, want one per tick", n)
	}
	if !strings.Contains(out, "4m30s remaining") {
		t.Fatalf("expected the injected clock's remaining time, got %q", out)
	}
	if n := strings.Count(out, "open in a browser on this machine:"); n != 2 {
		t.Fatalf("every tick must carry the machine qualifier; the line is read in isolation. got %d in %q",
			n, out)
	}
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("expected exactly one line per tick, got %q", out)
	}
}

func TestWaitRendererTTYModeAnimatesInPlaceAndClearsOnDone(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0).UTC()
	deadline := t0.Add(5 * time.Minute)
	var buf bytes.Buffer

	r, interval := newWaitRenderer(&buf, "https://auth.example.com/authorize?x=1", deadline,
		func() time.Time { return t0.Add(90 * time.Second) }, true)
	if interval != waitAnimationInterval {
		t.Fatalf("interval = %v, want %v", interval, waitAnimationInterval)
	}
	r.onTick()
	r.onTick()
	r.onDone()

	out := buf.String()
	if strings.Count(out, "\r") < 3 {
		t.Fatalf("expected in-place redraws, got %q", out)
	}
	if !strings.Contains(out, "\x1b[K") {
		t.Fatalf("expected erase-to-EOL, got %q", out)
	}
	if !strings.Contains(out, "remaining: 3m30s") {
		t.Fatalf("expected the injected clock's remaining time, got %q", out)
	}
	if !strings.HasSuffix(out, "\r\x1b[K") {
		t.Fatalf("onDone must clear the line, got %q", out)
	}
}

func TestWaitRendererRemainingNeverGoesNegative(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0).UTC()
	if got := remaining(t0, func() time.Time { return t0.Add(time.Hour) }); got != 0 {
		t.Fatalf("remaining = %v, want 0 past the deadline", got)
	}
}
