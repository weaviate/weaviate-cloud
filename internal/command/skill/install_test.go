package skill_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/guide"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

// newSkillFactory builds a test factory with a temp HOME. The caller may set
// stdinTTY to true to test the interactive path.
func newSkillFactory(t *testing.T, stdinInput string, stdinTTY bool) (*factory.Factory, *bytes.Buffer) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv("USERPROFILE", tmp)

	ios, in, out, _ := iostreams.Test()
	if stdinInput != "" {
		in.WriteString(stdinInput)
	}
	if stdinTTY {
		ios.SetStdinTTY(true)
	}

	f := &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "req-fixed" },
	}
	return f, out
}

func runSkill(t *testing.T, f *factory.Factory, args ...string) error {
	t.Helper()
	root := cli.NewRootCmd(f)
	root.SetArgs(args)
	root.SetIn(f.IOStreams.In)
	root.SetOut(f.IOStreams.Out)
	root.SetErr(f.IOStreams.Err)
	return root.ExecuteContext(context.Background())
}

// ── Guard / flag tests ──────────────────────────────────────────────────────

//nolint:paralleltest // t.Setenv
func TestInstallNonTTYNoFlagsErrors(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)
	err := runSkill(t, f, "skill", "install", "-o", "json")
	if err == nil {
		t.Fatal("expected error for non-TTY with no selection flags")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errcode.Error", err)
	}
	if e.Code != errcode.CodeValidationFailed {
		t.Errorf("code = %q, want validation_failed", e.Code)
	}
	if !strings.Contains(e.Message, "--all") && !strings.Contains(e.Message, "--harness") {
		t.Errorf("message = %q, want mention of --all or --harness", e.Message)
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallAllAndHarnessConflict(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)
	err := runSkill(t, f, "skill", "install", "--all", "--harness", "codex", "-o", "json")
	if err == nil {
		t.Fatal("expected error for --all + --harness combination")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errcode.Error", err)
	}
	if e.Code != errcode.CodeValidationFailed {
		t.Errorf("code = %q, want validation_failed", e.Code)
	}
	if !strings.Contains(e.Message, "--all") || !strings.Contains(e.Message, "--harness") {
		t.Errorf("message = %q, want mention of both --all and --harness", e.Message)
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallUnknownHarness(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)
	err := runSkill(t, f, "skill", "install", "--harness", "bogus", "-o", "json")
	if err == nil {
		t.Fatal("expected error for unknown harness")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errcode.Error", err)
	}
	if e.Code != errcode.CodeValidationFailed {
		t.Errorf("code = %q, want validation_failed", e.Code)
	}
	if !strings.Contains(e.Message, "bogus") {
		t.Errorf("message = %q, want it to name the unknown harness", e.Message)
	}
	// Must list accepted values.
	for _, known := range []string{"claude-code", "codex", "cursor", "gemini-cli", "copilot", "opencode"} {
		if !strings.Contains(e.Message, known) {
			t.Errorf("message = %q, want accepted value %q listed", e.Message, known)
		}
	}
	// No files written.
	tmp := os.Getenv("HOME")
	for _, dir := range []string{".agents", ".claude"} {
		if _, err2 := os.Stat(filepath.Join(tmp, dir, "skills")); !os.IsNotExist(err2) {
			t.Errorf("files written despite validation error: %s", filepath.Join(tmp, dir))
		}
	}
}

// ── Write tests ─────────────────────────────────────────────────────────────

//nolint:paralleltest // t.Setenv
func TestInstallAllWritesTargets(t *testing.T) {
	f, stdout := newSkillFactory(t, "", false)
	tmp := os.Getenv("HOME")

	err := runSkill(t, f, "skill", "install", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("skill install --all: %v", err)
	}

	// Both expected paths must exist and be non-empty.
	expectedPaths := []string{
		filepath.Join(tmp, ".agents", "skills", "wcloud", "SKILL.md"),
		filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md"),
	}
	for _, p := range expectedPaths {
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			t.Errorf("expected file %s: %v", p, readErr)
			continue
		}
		if len(data) == 0 {
			t.Errorf("file %s is empty", p)
		}
		// Verify body == guide.Markdown() (single-source guarantee).
		assertSKILLBody(t, data)
	}

	// Decode JSON envelope.
	var env struct {
		Data struct {
			Installed []struct {
				Harness string `json:"harness"`
				Path    string `json:"path"`
			} `json:"installed"`
		} `json:"data"`
		Metadata struct {
			APIVersion string `json:"api_version"`
			RequestID  string `json:"request_id"`
		} `json:"metadata"`
	}
	if err2 := json.Unmarshal(stdout.Bytes(), &env); err2 != nil {
		t.Fatalf("decode envelope: %v\nstdout=%s", err2, stdout.String())
	}
	if env.Metadata.APIVersion != output.APIVersion {
		t.Errorf("api_version = %q, want %q", env.Metadata.APIVersion, output.APIVersion)
	}
	if env.Metadata.RequestID == "" {
		t.Error("request_id is empty")
	}
	if len(env.Data.Installed) == 0 {
		t.Error("data.installed is empty")
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallProjectWritesLocal(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)
	tmp := os.Getenv("HOME")

	// Use a separate temp dir as project root.
	projDir := t.TempDir()
	t.Chdir(projDir)

	if err3 := runSkill(t, f, "skill", "install", "--all", "--project", "-o", "json"); err3 != nil {
		t.Fatalf("skill install --all --project: %v", err3)
	}

	// Project-local paths must exist.
	for _, rel := range []string{
		filepath.Join(".agents", "skills", "wcloud", "SKILL.md"),
		filepath.Join(".claude", "skills", "wcloud", "SKILL.md"),
	} {
		if _, statErr := os.Stat(filepath.Join(projDir, rel)); statErr != nil {
			t.Errorf("project path %s not created: %v", rel, statErr)
		}
	}

	// Global paths must NOT exist.
	for _, dir := range []string{".agents", ".claude"} {
		p := filepath.Join(tmp, dir, "skills")
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("global path %s was written but should not be", p)
		}
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallHarnessSubset(t *testing.T) {
	t.Run("claude-code only", func(t *testing.T) {
		//nolint:paralleltest // t.Setenv
		f, _ := newSkillFactory(t, "", false)
		tmp := os.Getenv("HOME")

		if err := runSkill(t, f, "skill", "install", "--harness", "claude-code", "-o", "json"); err != nil {
			t.Fatalf("skill install --harness claude-code: %v", err)
		}
		claudePath := filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md")
		if _, err := os.Stat(claudePath); err != nil {
			t.Errorf(".claude SKILL.md not created: %v", err)
		}
		agentsPath := filepath.Join(tmp, ".agents", "skills", "wcloud", "SKILL.md")
		if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
			t.Errorf(".agents SKILL.md created but should not be for --harness claude-code")
		}
	})

	t.Run("codex only", func(t *testing.T) {
		//nolint:paralleltest // t.Setenv
		f, _ := newSkillFactory(t, "", false)
		tmp := os.Getenv("HOME")

		if err := runSkill(t, f, "skill", "install", "--harness", "codex", "-o", "json"); err != nil {
			t.Fatalf("skill install --harness codex: %v", err)
		}
		agentsPath := filepath.Join(tmp, ".agents", "skills", "wcloud", "SKILL.md")
		if _, err := os.Stat(agentsPath); err != nil {
			t.Errorf(".agents SKILL.md not created: %v", err)
		}
		claudePath := filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md")
		if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
			t.Errorf(".claude SKILL.md created but should not be for --harness codex")
		}
	})

	t.Run("claude-code and codex together", func(t *testing.T) {
		//nolint:paralleltest // t.Setenv
		f, _ := newSkillFactory(t, "", false)
		tmp := os.Getenv("HOME")

		if err := runSkill(t, f, "skill", "install", "--harness", "claude-code,codex", "-o", "json"); err != nil {
			t.Fatalf("skill install --harness claude-code,codex: %v", err)
		}
		for _, p := range []string{
			filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md"),
			filepath.Join(tmp, ".agents", "skills", "wcloud", "SKILL.md"),
		} {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("expected %s: %v", p, err)
			}
		}
	})
}

//nolint:paralleltest // t.Setenv
func TestInstallIdempotent(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)

	for i := range 2 {
		if err := runSkill(t, f, "skill", "install", "--all", "-o", "json"); err != nil {
			t.Fatalf("run %d: skill install --all: %v", i+1, err)
		}
	}

	tmp := os.Getenv("HOME")
	for _, p := range []string{
		filepath.Join(tmp, ".agents", "skills", "wcloud", "SKILL.md"),
		filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md"),
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("read %s: %v", p, err)
			continue
		}
		// Single-source guarantee: body must equal guide.Markdown().
		assertSKILLBody(t, data)
		// Idempotent = starts with exactly one frontmatter block.
		if !strings.HasPrefix(string(data), "---\n") {
			t.Errorf("%s: does not start with ---", p)
		}
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallRetightensLoosenedFile(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)
	tmp := os.Getenv("HOME")
	path := filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md")

	if err := runSkill(t, f, "skill", "install", "--harness", "claude-code", "-o", "json"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("pre-loosen: %v", err)
	}

	if err := runSkill(t, f, "skill", "install", "--harness", "claude-code", "-o", "json"); err != nil {
		t.Fatalf("second install: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("perm = %o, want 0600 (re-tightened)", mode)
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallRetightensLoosenedDir(t *testing.T) {
	f, _ := newSkillFactory(t, "", false)
	tmp := os.Getenv("HOME")
	dir := filepath.Join(tmp, ".claude", "skills", "wcloud")

	if err := runSkill(t, f, "skill", "install", "--harness", "claude-code", "-o", "json"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("pre-loosen dir: %v", err)
	}

	if err := runSkill(t, f, "skill", "install", "--harness", "claude-code", "-o", "json"); err != nil {
		t.Fatalf("second install: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("dir perm = %o, want 0700 (re-tightened)", mode)
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallTextOutput(t *testing.T) {
	f, stdout := newSkillFactory(t, "", false)

	if err := runSkill(t, f, "skill", "install", "--all", "-o", "text"); err != nil {
		t.Fatalf("skill install --all -o text: %v", err)
	}
	got := stdout.String()
	if strings.Contains(got, "request_id") {
		t.Errorf("text output must not contain request_id, got: %s", got)
	}
	// Should contain some harness output.
	if !strings.Contains(got, "wcloud") && !strings.Contains(got, "SKILL.md") {
		t.Errorf("text output = %q, want some harness/path info", got)
	}
}

//nolint:paralleltest // t.Setenv
func TestInstallInteractivePreselectsDetected(t *testing.T) {
	// Create .claude footprint so claude-code is detected and pre-selected.
	f, _ := newSkillFactory(t, "\n", true) // "\n" = accept defaults
	tmp := os.Getenv("HOME")
	if err := os.MkdirAll(filepath.Join(tmp, ".claude"), 0o700); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	if err := runSkill(t, f, "skill", "install", "-o", "json"); err != nil {
		t.Fatalf("interactive install: %v", err)
	}

	// claude-code was detected and pre-selected → SKILL.md must exist.
	claudePath := filepath.Join(tmp, ".claude", "skills", "wcloud", "SKILL.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf(".claude SKILL.md not written after interactive accept: %v", err)
	}
	assertSKILLBody(t, data)

	// .agents should not be written (codex was not detected).
	agentsPath := filepath.Join(tmp, ".agents", "skills", "wcloud", "SKILL.md")
	if _, err2 := os.Stat(agentsPath); !os.IsNotExist(err2) {
		t.Errorf(".agents SKILL.md written but should not be (codex not detected)")
	}
}

// assertSKILLBody verifies the SKILL.md file has valid frontmatter and that
// its body equals guide.Markdown() (the single-source guarantee).
func assertSKILLBody(t *testing.T, data []byte) {
	t.Helper()
	s := string(data)

	if !strings.HasPrefix(s, "---\n") {
		t.Errorf("SKILL.md does not start with frontmatter: %.40s", s)
	}
	if !strings.Contains(s, "name: wcloud") {
		t.Error("SKILL.md missing name: wcloud")
	}

	parts := strings.SplitN(s, "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("SKILL.md has no closing frontmatter delimiter")
	}
	body := strings.TrimPrefix(parts[1], "\n")
	want := guide.Markdown()
	if body != want {
		t.Errorf("SKILL.md body differs from guide.Markdown(): body len=%d want len=%d", len(body), len(want))
	}
}
