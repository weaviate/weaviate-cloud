package iostreams

import (
	"bytes"
	"io"
	"os"
)

type IOStreams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	stdoutIsTTY bool
	stderrIsTTY bool
	stdinIsTTY  bool
}

func System() *IOStreams {
	return &IOStreams{
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		stdinIsTTY:  isTerminal(os.Stdin),
		stdoutIsTTY: isTerminal(os.Stdout),
		stderrIsTTY: isTerminal(os.Stderr),
	}
}

func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	return &IOStreams{In: in, Out: out, Err: errBuf}, in, out, errBuf
}

func (s *IOStreams) IsStdoutTTY() bool { return s.stdoutIsTTY }
func (s *IOStreams) IsStderrTTY() bool { return s.stderrIsTTY }
func (s *IOStreams) IsStdinTTY() bool  { return s.stdinIsTTY }

func (s *IOStreams) SetStdinTTY(v bool)  { s.stdinIsTTY = v }
func (s *IOStreams) SetStdoutTTY(v bool) { s.stdoutIsTTY = v }
func (s *IOStreams) SetStderrTTY(v bool) { s.stderrIsTTY = v }
