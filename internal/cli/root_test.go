package cli_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func TestRootCmd_OutputFormat_AutoNonTTY(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if f.OutputFormat != output.FormatJSON {
		t.Errorf("OutputFormat = %v, want FormatJSON", f.OutputFormat)
	}
}

func TestRootCmd_OutputFormat_ExplicitText(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if f.OutputFormat != output.FormatText {
		t.Errorf("OutputFormat = %v, want FormatText", f.OutputFormat)
	}
}

func TestRootGlobalFlagsUnchanged(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}
	root := cli.NewRootCmd(f)
	want := map[string]bool{"output": true, "verbose": true}
	for name := range want {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("expected global flag --%s is missing", name)
		}
	}
	// Verify no unexpected persistent flags were added.
	count := 0
	root.PersistentFlags().VisitAll(func(_ *pflag.Flag) { count++ })
	if count != len(want) {
		t.Errorf("global persistent flag count = %d, want %d; check for unexpected new flags", count, len(want))
	}
}

func TestLoginAliasHasTheSameFlagsAsAuthLogin(t *testing.T) {
	t.Parallel()

	streams, _, _, _ := iostreams.Test()
	root := cli.NewRootCmd(&factory.Factory{IOStreams: streams})

	alias, _, err := root.Find([]string{"login"})
	if err != nil {
		t.Fatalf("find login: %v", err)
	}
	full, _, err := root.Find([]string{"auth", "login"})
	if err != nil {
		t.Fatalf("find auth login: %v", err)
	}

	for _, name := range []string{"timeout", "no-launch-browser"} {
		a := alias.Flags().Lookup(name)
		b := full.Flags().Lookup(name)
		if a == nil || b == nil {
			t.Fatalf("flag %q: alias=%v auth=%v, both must exist", name, a != nil, b != nil)
		}
		if a.DefValue != b.DefValue || a.Usage != b.Usage {
			t.Fatalf("flag %q diverges: alias(%q,%q) vs auth(%q,%q)",
				name, a.DefValue, a.Usage, b.DefValue, b.Usage)
		}
	}
}

func TestRootCmd_Verbose_PropagatesToFactory(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}
	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--verbose", "version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !f.Verbose {
		t.Error("f.Verbose = false, want true when --verbose is passed")
	}
}

func TestRootCmd_Verbose_DefaultsFalse(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}
	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if f.Verbose {
		t.Error("f.Verbose = true, want false by default")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestArgValidationErrors_MapToUsageError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"cluster status no id", []string{"cluster", "status"}},
		{"cluster get no id", []string{"cluster", "get"}},
		{"cluster list extra arg", []string{"cluster", "list", "extra"}},
		{"cluster create extra arg", []string{"cluster", "create", "extra"}},
		{"region list extra arg", []string{"region", "list", "extra"}},
		{"skill install extra arg", []string{"skill", "install", "extra"}},
		{"profile use no name", []string{"profile", "use"}},
		{"profile delete no name", []string{"profile", "delete"}},
		{"profile create too many args", []string{"profile", "create", "a", "b"}},
		{"profile show too many args", []string{"profile", "show", "a", "b"}},
		{"guide extra arg", []string{"guide", "extra"}},
		{"version extra arg", []string{"version", "extra"}},
		{"auth login extra arg", []string{"auth", "login", "extra"}},
		{"auth logout extra arg", []string{"auth", "logout", "extra"}},
		{"auth whoami extra arg", []string{"auth", "whoami", "extra"}},
		{"profile list extra arg", []string{"profile", "list", "extra"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _, _ := cmdtest.NewFactory(t, true)
			err := cmdtest.Run(t, f, tc.args...)
			if err == nil {
				t.Fatalf("expected an arg-validation error for %v", tc.args)
			}
			code, ok := errcode.CodeFor(err)
			if !ok || code != errcode.CodeValidationFailed {
				t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeValidationFailed)
			}
			if exit := errcode.ExitCodeFor(err); exit != errcode.UsageError {
				t.Fatalf("exit code = %d, want %d (UsageError)", exit, errcode.UsageError)
			}
		})
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestUnknownCommandsAndFlags_MapToUsageError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown root command", []string{"frobnicate"}},
		{"cluster delete", []string{"cluster", "delete", "cid-1"}},
		{"cluster suspend", []string{"cluster", "suspend"}},
		{"auth refresh", []string{"auth", "refresh"}},
		{"auth token", []string{"auth", "token"}},
		{"region delete", []string{"region", "delete", "eu-west-1"}},
		{"profile rename", []string{"profile", "rename", "dev"}},
		{"skill uninstall", []string{"skill", "uninstall"}},
		{"unknown root flag", []string{"--bogus"}},
		{"unknown verb flag", []string{"cluster", "list", "--bogus"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _, stdout := cmdtest.NewFactory(t, true)
			err := cmdtest.Run(t, f, tc.args...)
			if err == nil {
				t.Fatalf("expected an error for %v, got nil (stdout=%q)", tc.args, stdout.String())
			}
			code, ok := errcode.CodeFor(err)
			if !ok || code != errcode.CodeValidationFailed {
				t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeValidationFailed)
			}
			if exit := errcode.ExitCodeFor(err); exit != errcode.UsageError {
				t.Fatalf("exit code = %d, want %d (UsageError)", exit, errcode.UsageError)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout must stay empty on an unknown command, got help text: %q", stdout.String())
			}
		})
	}
}

func TestCommandGroupWithoutVerb_PrintsHelpToStderrUnderJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range groupsWithoutVerb() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ios, _, stdout, stderr := iostreams.Test()
			f := testFactory(ios)
			root := cli.NewRootCmd(f)
			root.SetArgs(append(tc.args, "-o", "json"))

			if err := root.Execute(); err != nil {
				t.Fatalf("%v must print help and succeed, got %v", tc.args, err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("%v polluted the JSON channel with help text, stdout = %q", tc.args, stdout.String())
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("%v printed no help, stderr = %q", tc.args, stderr.String())
			}
		})
	}
}

func TestCommandGroupWithoutVerb_PrintsHelpToStdoutUnderText(t *testing.T) {
	t.Parallel()
	for _, tc := range groupsWithoutVerb() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ios, _, stdout, _ := iostreams.Test()
			f := testFactory(ios)
			root := cli.NewRootCmd(f)
			root.SetArgs(append(tc.args, "-o", "text"))

			if err := root.Execute(); err != nil {
				t.Fatalf("%v must print help and succeed, got %v", tc.args, err)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("%v printed no help, stdout = %q", tc.args, stdout.String())
			}
		})
	}
}

func TestExplicitHelpRequestStaysOnStdout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"root --help", []string{"--help"}},
		{"group --help", []string{"cluster", "--help"}},
		{"verb --help", []string{"cluster", "get", "--help"}},
		{"help subcommand", []string{"help", "cluster"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ios, _, stdout, stderr := iostreams.Test()
			f := testFactory(ios)
			root := cli.NewRootCmd(f)
			root.SetArgs(append(tc.args, "-o", "json"))

			if err := root.Execute(); err != nil {
				t.Fatalf("%v must succeed, got %v", tc.args, err)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("%v must keep help on stdout, stdout = %q stderr = %q",
					tc.args, stdout.String(), stderr.String())
			}
		})
	}
}

func groupsWithoutVerb() []struct {
	name string
	args []string
} {
	return []struct {
		name string
		args []string
	}{
		{"root", nil},
		{"cluster", []string{"cluster"}},
		{"auth", []string{"auth"}},
		{"region", []string{"region"}},
		{"profile", []string{"profile"}},
		{"skill", []string{"skill"}},
	}
}

func testFactory(ios *iostreams.IOStreams) *factory.Factory {
	return &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}
}

func TestSkillVisibleInHelp(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}
	root := cli.NewRootCmd(f)
	var buf bytes.Buffer
	root.SetOut(&buf)
	if err := root.Help(); err != nil {
		t.Fatalf("Help: %v", err)
	}
	helpText := buf.String()
	if !strings.Contains(helpText, "skill") {
		t.Errorf("root help does not list 'skill' command, got:\n%s", helpText)
	}
	if !strings.Contains(helpText, "guide") {
		t.Errorf("root help does not list 'guide' command, got:\n%s", helpText)
	}
}
