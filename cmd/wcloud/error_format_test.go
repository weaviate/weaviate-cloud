package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

//nolint:paralleltest // humanAtATerminal uses t.Setenv
func TestErrorFormat_SubCaseB_ArgValidation_TTY_RendersText(t *testing.T) {
	humanAtATerminal(t)
	ios, _, stdout, stderr := iostreams.Test()
	ios.SetStdoutTTY(true)
	f := &factory.Factory{IOStreams: ios, NewRequestID: func() string { return "req-test" }}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"cluster", "status"}) // ExactArgs(1): fails before PersistentPreRunE

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an arg-validation error, got nil")
	}

	resolveErrorFormat(f, root)
	if f.OutputFormat != output.FormatText {
		t.Fatalf("OutputFormat = %v, want FormatText", f.OutputFormat)
	}

	writeErrorEnvelope(f, err)

	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty in text mode, got: %s", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "Error: accepts 1 arg(s), received 0") {
		t.Fatalf("stderr = %q, want it to contain the text error line", got)
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestErrorFormat_SubCaseB_ArgValidation_ExplicitOutputText_RendersText(t *testing.T) {
	ios, _, stdout, stderr := iostreams.Test() // non-TTY; --output text must still win
	f := &factory.Factory{IOStreams: ios, NewRequestID: func() string { return "req-test" }}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "status"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an arg-validation error, got nil")
	}

	resolveErrorFormat(f, root)
	if f.OutputFormat != output.FormatText {
		t.Fatalf("OutputFormat = %v, want FormatText", f.OutputFormat)
	}

	writeErrorEnvelope(f, err)

	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty in text mode, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error: accepts 1 arg(s), received 0") {
		t.Fatalf("stderr = %q, want it to contain the text error line", stderr.String())
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestErrorFormat_SubCaseB_UnknownCommand_TTY_RendersText(t *testing.T) {
	humanAtATerminal(t)
	ios, _, stdout, stderr := iostreams.Test()
	ios.SetStdoutTTY(true)
	f := &factory.Factory{IOStreams: ios, NewRequestID: func() string { return "req-test" }}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"badcommand"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an unknown-command error, got nil")
	}

	resolveErrorFormat(f, root)
	if f.OutputFormat != output.FormatText {
		t.Fatalf("OutputFormat = %v, want FormatText", f.OutputFormat)
	}

	writeErrorEnvelope(f, err)

	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty in text mode, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Fatalf("stderr = %q, want it to contain a text Error: line", stderr.String())
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestErrorFormat_SubCaseB_BadOutputFlag_FallsBackToTTYDefault(t *testing.T) {
	humanAtATerminal(t)
	ios, _, stdout, stderr := iostreams.Test()
	ios.SetStdoutTTY(true)
	f := &factory.Factory{IOStreams: ios, NewRequestID: func() string { return "req-test" }}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "bogus", "version"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected a --output validation error, got nil")
	}

	resolveErrorFormat(f, root)
	if f.OutputFormat != output.FormatText {
		t.Fatalf("OutputFormat = %v, want FormatText (TTY fallback since the flag itself is invalid)",
			f.OutputFormat)
	}

	writeErrorEnvelope(f, err)

	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty in text mode, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Fatalf("stderr = %q, want it to contain a text Error: line", stderr.String())
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestErrorFormat_SubCaseB_ArgValidation_NonTTY_StaysJSON(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test() // non-TTY by default

	f := &factory.Factory{IOStreams: ios, NewRequestID: func() string { return "req-test" }}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"cluster", "status"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an arg-validation error, got nil")
	}

	resolveErrorFormat(f, root)
	if f.OutputFormat != output.FormatJSON {
		t.Fatalf("OutputFormat = %v, want FormatJSON", f.OutputFormat)
	}

	writeErrorEnvelope(f, err)

	var m map[string]any
	if jsonErr := json.Unmarshal(stdout.Bytes(), &m); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw=%s", jsonErr, stdout.Bytes())
	}

	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("stdout must have an 'error' object, got: %s", stdout.Bytes())
	}
	if _, codeOK := errObj["code"].(string); !codeOK {
		t.Fatalf("error.code missing or wrong type: %v", errObj)
	}
	if _, msgOK := errObj["message"].(string); !msgOK {
		t.Fatalf("error.message missing or wrong type: %v", errObj)
	}
	meta, ok := m["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("stdout must have a 'metadata' object, got: %s", stdout.Bytes())
	}
	if v, _ := meta["api_version"].(string); v == "" {
		t.Errorf("metadata.api_version is empty, want non-empty")
	}
	if v, _ := meta["request_id"].(string); v == "" {
		t.Errorf("metadata.request_id is empty, want non-empty")
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestErrorFormat_SubCaseA_AlreadyResolved_NotOverridden(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(false) // would resolve to JSON if re-resolved
	f := &factory.Factory{
		IOStreams:    ios,
		OutputFormat: output.FormatText, // PersistentPreRunE already resolved this to text
		NewRequestID: func() string { return "req-test" },
	}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"version"}) // never executed; only need a *cobra.Command to pass in

	resolveErrorFormat(f, root)
	if f.OutputFormat != output.FormatText {
		t.Fatalf("resolveErrorFormat must not override an already-resolved format; got %v", f.OutputFormat)
	}
}

// humanAtATerminal clears the ambient CI markers so a test that simulates a
// terminal asserts the human path, not the runner's.
func humanAtATerminal(t *testing.T) {
	t.Helper()
	t.Setenv("CI", "")
	t.Setenv("TERM", "xterm-256color")
}
