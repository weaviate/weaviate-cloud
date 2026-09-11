package auth

import (
	"fmt"
	"io"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/output"
)

const (
	waitLineInterval      = 15 * time.Second
	waitAnimationInterval = 120 * time.Millisecond
	spinnerFrames         = "|/-\\"
)

type waitRenderer interface {
	onTick()
	onDone()
}

type lineWaitRenderer struct {
	w        io.Writer
	url      string
	deadline time.Time
	nowFn    func() time.Time
}

func newLineWaitRenderer(w io.Writer, url string, deadline time.Time, nowFn func() time.Time) *lineWaitRenderer {
	return &lineWaitRenderer{w: w, url: url, deadline: deadline, nowFn: nowFn}
}

func (r *lineWaitRenderer) onTick() {
	fmt.Fprintf(r.w, "waiting for sign-in: %s remaining - open in a browser on this machine: %s\n",
		remaining(r.deadline, r.nowFn), r.url)
}

func (r *lineWaitRenderer) onDone() {}

// spinnerWaitRenderer is the TTY renderer. \x1b[K (erase to end of line) clears
// leftovers from a previous, longer countdown; output stays uncoloured.
type spinnerWaitRenderer struct {
	w        io.Writer
	deadline time.Time
	nowFn    func() time.Time
	frame    int
}

func newSpinnerWaitRenderer(w io.Writer, deadline time.Time, nowFn func() time.Time) *spinnerWaitRenderer {
	return &spinnerWaitRenderer{w: w, deadline: deadline, nowFn: nowFn}
}

func (r *spinnerWaitRenderer) onTick() {
	frame := spinnerFrames[r.frame%len(spinnerFrames)]
	r.frame++
	fmt.Fprintf(r.w, "\r%c waiting for sign-in  remaining: %s\x1b[K", frame, remaining(r.deadline, r.nowFn))
}

func (r *spinnerWaitRenderer) onDone() {
	fmt.Fprint(r.w, "\r\x1b[K")
}

func newWaitRenderer(
	w io.Writer, url string, deadline time.Time, nowFn func() time.Time, stderrTTY bool,
) (waitRenderer, time.Duration) {
	if stderrTTY {
		return newSpinnerWaitRenderer(w, deadline, nowFn), waitAnimationInterval
	}
	return newLineWaitRenderer(w, url, deadline, nowFn), waitLineInterval
}

func remaining(deadline time.Time, nowFn func() time.Time) time.Duration {
	return max(deadline.Sub(nowFn()), 0).Round(time.Second)
}

func printSignInURL(w io.Writer, authURL string) {
	fmt.Fprintf(w, "\nTo sign in, open this URL in a browser on this machine:\n\n  %s\n\n",
		output.SanitizeText(authURL))
}
