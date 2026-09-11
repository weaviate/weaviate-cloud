package profile

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// wizardIO writes prompts to errOut (stdout is reserved for the JSON
// envelope) and reads answers from in.
type wizardIO struct {
	in     *bufio.Reader
	errOut io.Writer
}

func newWizardIO(in io.Reader, errOut io.Writer) *wizardIO {
	return &wizardIO{in: bufio.NewReader(in), errOut: errOut}
}

func (w *wizardIO) prompt(label, hint string) (string, error) {
	if hint != "" {
		fmt.Fprintf(w.errOut, "%s (%s): ", label, hint)
	} else {
		fmt.Fprintf(w.errOut, "%s: ", label)
	}
	line, err := w.in.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func (w *wizardIO) confirm(label string, defaultYes bool) (bool, error) {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	for {
		ans, err := w.prompt(label, hint)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(ans) {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		fmt.Fprintln(w.errOut, "please answer y or n")
	}
}
