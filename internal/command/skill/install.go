package skill

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type installResult struct {
	Installed []installedEntry `json:"installed"`
}

type installedEntry struct {
	Harness string `json:"harness"`
	Path    string `json:"path"`
}

func NewInstallCmd(f *factory.Factory) *cobra.Command {
	var (
		flagAll     bool
		flagHarness []string
		flagProject bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the wcloud SKILL.md into one or more coding harnesses",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstall(cmd.Context(), f, flagAll, flagHarness, flagProject)
		},
	}

	cmd.Flags().BoolVar(&flagAll, "all", false, "install to all supported harnesses")
	cmd.Flags().StringSliceVar(&flagHarness, "harness", nil,
		"comma-separated harness names: claude-code, codex, cursor, gemini-cli, copilot, opencode")
	cmd.Flags().BoolVar(&flagProject, "project", false,
		"write to project-local paths instead of user-global (default global)")

	return cmd
}

func runInstall(ctx context.Context, f *factory.Factory, flagAll bool, flagHarness []string, flagProject bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	selected, err := resolveSelected(ctx, f, home, flagAll, flagHarness)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return errcode.New(errcode.CodeValidationFailed, "no harnesses selected; nothing written")
	}

	paths := resolveTargets(selected, home, projectRoot, flagProject)
	content := renderSKILL()

	entries, err := writeSkills(paths, content)
	if err != nil {
		return err
	}

	env := output.Envelope[installResult]{
		Data:     installResult{Installed: entries},
		Metadata: makeMetadata(f),
	}
	return output.WriteEnv(f.IOStreams, f.OutputFormat, env, renderInstallText)
}

func resolveSelected(
	ctx context.Context,
	f *factory.Factory,
	home string,
	flagAll bool,
	flagHarness []string,
) ([]harness, error) {
	if flagAll && len(flagHarness) > 0 {
		return nil, errcode.New(errcode.CodeValidationFailed,
			"--all and --harness are mutually exclusive; use one or the other")
	}
	switch {
	case len(flagHarness) > 0:
		return parseHarnesses(flagHarness)
	case flagAll:
		out := make([]harness, len(knownHarnesses))
		copy(out, knownHarnesses)
		return out, nil
	case f.IOStreams.IsStdinTTY():
		detected := detectHarnesses(home)
		return interactiveSelect(ctx, f.IOStreams.In, f.IOStreams.Err, detected)
	default:
		return nil, errcode.New(errcode.CodeValidationFailed,
			"skill install requires --all or --harness when not run interactively; "+
				"use --all to install all supported harnesses or --harness <name> to specify one or more")
	}
}

func writeSkills(paths []string, content string) ([]installedEntry, error) {
	entries := make([]installedEntry, 0, len(paths))
	for _, p := range paths {
		if err := config.EnsureDir(filepath.Dir(p), config.DirPerm); err != nil {
			return nil, fmt.Errorf("create directory for %s: %w", p, err)
		}
		if err := config.WriteFileAtomic(p, []byte(content), config.FilePerm); err != nil {
			return nil, fmt.Errorf("write %s: %w", p, err)
		}
		entries = append(entries, installedEntry{Harness: harnessLabelForPath(p), Path: p})
	}
	return entries, nil
}

func parseHarnesses(raw []string) ([]harness, error) {
	known := make(map[harness]bool, len(knownHarnesses))
	for _, h := range knownHarnesses {
		known[h] = true
	}

	var result []harness
	var unknown []string
	for _, s := range raw {
		for part := range strings.SplitSeq(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			h := harness(part)
			if !known[h] {
				unknown = append(unknown, part)
			} else {
				result = append(result, h)
			}
		}
	}
	if len(unknown) > 0 {
		names := make([]string, len(knownHarnesses))
		for i, h := range knownHarnesses {
			names[i] = string(h)
		}
		return nil, errcode.New(errcode.CodeValidationFailed,
			fmt.Sprintf("unknown harness %q; accepted values: %s",
				strings.Join(unknown, ", "), strings.Join(names, ", ")))
	}
	return result, nil
}

// interactiveSelect renders a numbered list and reads the user's selection.
// Detected harnesses are pre-checked. An empty line accepts the defaults.
func interactiveSelect(ctx context.Context, in io.Reader, errOut io.Writer, detected []harness) ([]harness, error) {
	preSelected := make(map[harness]bool, len(detected))
	for _, h := range detected {
		preSelected[h] = true
	}

	fmt.Fprintln(errOut, "Select harnesses to install the wcloud skill into (detected harnesses are pre-selected):")
	for i, h := range knownHarnesses {
		mark := "[ ]"
		if preSelected[h] {
			mark = "[x]"
		}
		fmt.Fprintf(errOut, "  %d. %s %s\n", i+1, mark, h)
	}
	fmt.Fprintln(errOut, "Enter comma-separated numbers to toggle, or press Enter to accept defaults:")

	line, err := readLineOrCancel(ctx, in)
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)

	if line == "" {
		return defaultsFrom(preSelected), nil
	}
	return toggleFrom(preSelected, line, errOut), nil
}

func defaultsFrom(preSelected map[harness]bool) []harness {
	result := make([]harness, 0, len(preSelected))
	for _, h := range knownHarnesses {
		if preSelected[h] {
			result = append(result, h)
		}
	}
	return result
}

func toggleFrom(preSelected map[harness]bool, line string, errOut io.Writer) []harness {
	toggled := make(map[harness]bool, len(preSelected))
	maps.Copy(toggled, preSelected)
	for part := range strings.SplitSeq(line, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var idx int
		if _, scanErr := fmt.Sscan(part, &idx); scanErr != nil || idx < 1 || idx > len(knownHarnesses) {
			fmt.Fprintf(errOut, "ignoring invalid entry %q\n", part)
			continue
		}
		h := knownHarnesses[idx-1]
		toggled[h] = !toggled[h]
	}
	result := make([]harness, 0, len(toggled))
	for _, h := range knownHarnesses {
		if toggled[h] {
			result = append(result, h)
		}
	}
	return result
}

func harnessLabelForPath(p string) string {
	if strings.Contains(p, "/.claude/") || strings.Contains(p, "\\.claude\\") {
		return string(harnessClaudeCode)
	}
	return "agents"
}

func renderInstallText(w io.Writer, r installResult) error {
	rows := make([][]string, len(r.Installed))
	for i, e := range r.Installed {
		rows[i] = []string{e.Harness, e.Path}
	}
	return output.Table(w, []string{"HARNESS", "PATH"}, rows)
}

func makeMetadata(f *factory.Factory) output.Metadata {
	return output.Metadata{APIVersion: output.APIVersion, RequestID: f.NewRequestID()}
}

// WHY: the read goroutine is deliberately not joined — it stays parked on a stdin
// read that will never return, and the process is exiting anyway.
func readLineOrCancel(ctx context.Context, in io.Reader) (string, error) {
	type read struct {
		line string
		err  error
	}
	ch := make(chan read, 1)
	go func() {
		line, err := bufio.NewReader(in).ReadString('\n')
		ch <- read{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("skill install cancelled: %w", ctx.Err())
	case r := <-ch:
		if r.err != nil && (!errors.Is(r.err, io.EOF) || r.line == "") {
			return "", fmt.Errorf("read selection: %w", r.err)
		}
		return r.line, nil
	}
}
