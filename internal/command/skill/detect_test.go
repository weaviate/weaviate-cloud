//nolint:testpackage // white-box: accesses unexported harness detection symbols
package skill

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	copilotDir = ".copilot"
	vscodeDir  = ".vscode"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	home := t.TempDir()

	// Nothing present — no harnesses detected.
	got := detectHarnesses(home)
	if len(got) != 0 {
		t.Fatalf("empty home: got %v, want none", got)
	}

	// Add .claude → claude-code detected.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	got = detectHarnesses(home)
	if !slices.Contains(got, harnessClaudeCode) {
		t.Fatalf("expected claude-code after creating .claude, got %v", got)
	}
	if slices.Contains(got, harnessCodex) {
		t.Fatalf("codex should not be detected without .codex, got %v", got)
	}

	// Add .codex → codex detected too.
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}
	got = detectHarnesses(home)
	if !slices.Contains(got, harnessClaudeCode) || !slices.Contains(got, harnessCodex) {
		t.Fatalf("expected claude-code + codex, got %v", got)
	}

	// Add .gemini → gemini-cli detected.
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o700); err != nil {
		t.Fatalf("mkdir .gemini: %v", err)
	}
	if !slices.Contains(detectHarnesses(home), harnessGeminiCLI) {
		t.Fatalf("expected gemini-cli after creating .gemini")
	}

	// Add .copilot → copilot detected.
	if err := os.MkdirAll(filepath.Join(home, copilotDir), 0o700); err != nil {
		t.Fatalf("mkdir .copilot: %v", err)
	}
	if !slices.Contains(detectHarnesses(home), harnessCopilot) {
		t.Fatalf("expected copilot after creating .copilot")
	}

	// Add .cursor → cursor detected.
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o700); err != nil {
		t.Fatalf("mkdir .cursor: %v", err)
	}
	if !slices.Contains(detectHarnesses(home), harnessCursor) {
		t.Fatalf("expected cursor after creating .cursor")
	}

	// .agents is shared by Codex, Cursor and Copilot, so it must not detect opencode.
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o700); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	if slices.Contains(detectHarnesses(home), harnessOpenCode) {
		t.Fatalf("opencode must not be detected from .agents")
	}

	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o700); err != nil {
		t.Fatalf("mkdir .config/opencode: %v", err)
	}
	if !slices.Contains(detectHarnesses(home), harnessOpenCode) {
		t.Fatalf("expected opencode after creating .config/opencode")
	}
}

func TestDetectOrderMatchesKnownHarnesses(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	for _, dir := range []string{
		".claude", ".codex", ".cursor", ".gemini", copilotDir, filepath.Join(".config", "opencode"),
		filepath.Join(".gemini", "antigravity-cli"), filepath.Join(".codeium", "windsurf"),
	} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	got := detectHarnesses(home)
	if !slices.Equal(got, knownHarnesses) {
		t.Fatalf("detected %v, want knownHarnesses %v in order", got, knownHarnesses)
	}
}

func TestDetectFootprints(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dirs []string
		want []harness
	}{
		{"empty home", nil, nil},
		{"agents alone is no harness", []string{".agents"}, nil},
		{".config alone is no harness", []string{".config"}, nil},
		{"opencode own folder", []string{filepath.Join(".config", "opencode")}, []harness{harnessOpenCode}},
		{"copilot cli folder", []string{copilotDir}, []harness{harnessCopilot}},
		{"vs code folder", []string{vscodeDir}, []harness{harnessCopilot}},
		{"both copilot folders report copilot once", []string{copilotDir, vscodeDir}, []harness{harnessCopilot}},
		{"gemini folder alone is gemini-cli only", []string{".gemini"}, []harness{harnessGeminiCLI}},
		{
			"antigravity cli folder also shows its gemini parent",
			[]string{filepath.Join(".gemini", "antigravity-cli")},
			[]harness{harnessGeminiCLI, harnessAntigravity},
		},
		{".codeium alone is no harness", []string{".codeium"}, nil},
		{"windsurf folder", []string{filepath.Join(".codeium", "windsurf")}, []harness{harnessDevin}},
		{"devin cli folder", []string{filepath.Join(".config", "devin")}, []harness{harnessDevin}},
		{
			"both devin folders report devin once",
			[]string{filepath.Join(".codeium", "windsurf"), filepath.Join(".config", "devin")},
			[]harness{harnessDevin},
		},
		{
			"opencode and devin share .config",
			[]string{filepath.Join(".config", "opencode"), filepath.Join(".config", "devin")},
			[]harness{harnessOpenCode, harnessDevin},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			for _, d := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(home, d), 0o700); err != nil {
					t.Fatalf("mkdir %s: %v", d, err)
				}
			}
			if got := detectHarnesses(home); !slices.Equal(got, tc.want) {
				t.Fatalf("detectHarnesses = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFootprintsAlignWithKnownHarnesses(t *testing.T) {
	t.Parallel()

	if len(footprints) != len(knownHarnesses) {
		t.Fatalf("len(footprints) = %d, want %d", len(footprints), len(knownHarnesses))
	}
	for i, fp := range footprints {
		if fp.h != knownHarnesses[i] {
			t.Errorf("footprints[%d] = %q, want %q (same order as knownHarnesses)", i, fp.h, knownHarnesses[i])
		}
		if len(fp.dirs) == 0 {
			t.Errorf("footprints[%d] (%q) has no directories", i, fp.h)
		}
	}
}
