package cluster

import (
	"fmt"
	"io"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

const (
	heartbeatEveryTicks      = 10
	defaultAnimationInterval = 120 * time.Millisecond
	spinnerFrames            = "|/-\\"
)

// progressRenderer lets pollUntilReady drive either rendering mode without
// branching on stderrTTY at every call site.
type progressRenderer interface {
	onPoll(status api.ClusterStatus)
	onTick()
	onDone(status api.ClusterStatus)
}

// lineRenderer is the non-TTY renderer — output must stay parse-clean for
// agent/CI consumers (the primary persona).
type lineRenderer struct {
	w          io.Writer
	lastStatus api.ClusterStatus
	hasEmitted bool
	sinceEmit  int
}

func newLineRenderer(w io.Writer) *lineRenderer {
	return &lineRenderer{w: w}
}

func (p *lineRenderer) onPoll(status api.ClusterStatus) {
	if p.w == nil {
		return
	}
	if !p.hasEmitted || status != p.lastStatus {
		fmt.Fprintf(p.w, "status: %s\n", output.SanitizeText(string(status)))
		p.lastStatus = status
		p.hasEmitted = true
		p.sinceEmit = 0
		return
	}
	p.sinceEmit++
	if p.sinceEmit >= heartbeatEveryTicks {
		fmt.Fprintf(p.w, "status: %s\n", output.SanitizeText(string(status)))
		p.sinceEmit = 0
	}
}

func (p *lineRenderer) onTick()                    {}
func (p *lineRenderer) onDone(_ api.ClusterStatus) {}

// spinnerRenderer is the TTY renderer. \x1b[K (erase to end of line) clears
// any leftover characters from a previous, longer status word; output stays
// uncoloured.
type spinnerRenderer struct {
	w      io.Writer
	start  time.Time
	nowFn  func() time.Time
	frame  int
	status api.ClusterStatus
}

func newSpinnerRenderer(w io.Writer, nowFn func() time.Time) *spinnerRenderer {
	return &spinnerRenderer{w: w, start: nowFn(), nowFn: nowFn}
}

func (s *spinnerRenderer) elapsed() time.Duration {
	return s.nowFn().Sub(s.start).Round(time.Second)
}

func (s *spinnerRenderer) redraw() {
	frame := spinnerFrames[s.frame%len(spinnerFrames)]
	fmt.Fprintf(s.w, "\r%c status: %s  elapsed: %s\x1b[K", frame, output.SanitizeText(string(s.status)), s.elapsed())
}

func (s *spinnerRenderer) onPoll(status api.ClusterStatus) {
	if s.w == nil {
		return
	}
	s.status = status
	s.frame++
	s.redraw()
}

func (s *spinnerRenderer) onTick() {
	if s.w == nil {
		return
	}
	s.frame++
	s.redraw()
}

func (s *spinnerRenderer) onDone(status api.ClusterStatus) {
	if s.w == nil {
		return
	}
	s.status = status
	fmt.Fprintf(s.w, "\rstatus: %s  elapsed: %s\x1b[K\n", output.SanitizeText(string(status)), s.elapsed())
}

func newProgressRenderer(w io.Writer, nowFn func() time.Time, stderrTTY bool) progressRenderer {
	if stderrTTY {
		return newSpinnerRenderer(w, nowFn)
	}
	return newLineRenderer(w)
}
