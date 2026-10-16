//nolint:testpackage // white-box: accesses unexported harness type and target resolution symbols
package skill

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestKnownHarnesses(t *testing.T) {
	t.Parallel()
	want := []harness{
		harnessClaudeCode,
		harnessCodex,
		harnessCursor,
		harnessGeminiCLI,
		harnessCopilot,
		harnessOpenCode,
		harnessAntigravity,
		harnessDevin,
	}
	if !slices.Equal(knownHarnesses, want) {
		t.Fatalf("knownHarnesses = %v, want %v (the first six keep their picker numbers)", knownHarnesses, want)
	}
	for _, h := range knownHarnesses {
		if h == harnessAliasWindsurf {
			t.Error("windsurf is an alias of devin and must not be its own harness")
		}
	}
}

func TestTargetPaths(t *testing.T) {
	t.Parallel()
	home := "/h"
	proj := "/p"

	cases := []struct {
		h           harness
		globalWant  string
		projectWant string
	}{
		{
			harnessClaudeCode,
			filepath.Join(home, ".claude", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".claude", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessCodex,
			filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessCursor,
			filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessGeminiCLI,
			filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessCopilot,
			filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessOpenCode,
			filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessAntigravity,
			filepath.Join(home, ".gemini", "antigravity-cli", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
		{
			harnessDevin,
			filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md"),
			filepath.Join(proj, ".agents", "skills", "wcloud", "SKILL.md"),
		},
	}

	for _, tc := range cases {
		got := targetPath(tc.h, home, proj, false)
		if got != tc.globalWant {
			t.Errorf("targetPath(%q, global): got %q, want %q", tc.h, got, tc.globalWant)
		}
		got = targetPath(tc.h, home, proj, true)
		if got != tc.projectWant {
			t.Errorf("targetPath(%q, project): got %q, want %q", tc.h, got, tc.projectWant)
		}
	}
}

func TestResolveTargetsDedupes(t *testing.T) {
	t.Parallel()
	home := "/h"
	proj := "/p"

	// codex and cursor both map to the same .agents path globally — should dedupe to one entry.
	paths := resolveTargets([]harness{harnessCodex, harnessCursor}, home, proj, false)
	if len(paths) != 1 {
		t.Fatalf("resolveTargets(codex,cursor) global = %v (len %d), want 1 path", paths, len(paths))
	}
	want := filepath.Join(home, ".agents", "skills", "wcloud", "SKILL.md")
	if paths[0] != want {
		t.Errorf("path = %q, want %q", paths[0], want)
	}
}

func TestResolveTargetsClaudeAndAgents(t *testing.T) {
	t.Parallel()
	home := "/h"
	proj := "/p"

	// claude-code + codex should yield two distinct paths.
	paths := resolveTargets([]harness{harnessClaudeCode, harnessCodex}, home, proj, false)
	if len(paths) != 2 {
		t.Fatalf("resolveTargets(claude-code,codex) = %v (len %d), want 2", paths, len(paths))
	}
}

func TestResolveTargetsDevinFoldsIntoAgents(t *testing.T) {
	t.Parallel()

	paths := resolveTargets([]harness{harnessCodex, harnessDevin, harnessAntigravity}, "/h", "/p", false)
	want := []string{
		filepath.Join("/h", ".agents", "skills", "wcloud", "SKILL.md"),
		filepath.Join("/h", ".gemini", "antigravity-cli", "skills", "wcloud", "SKILL.md"),
	}
	if !slices.Equal(paths, want) {
		t.Fatalf("resolveTargets = %v, want %v", paths, want)
	}
}

func TestLoadHints(t *testing.T) {
	t.Parallel()

	const fallback = "If the skill does not appear, have the agent read the SKILL.md at the reported path directly."
	cases := []struct {
		h     harness
		wants []string
	}{
		{harnessClaudeCode, []string{"/reload-skills", "did not exist before this install"}},
		{harnessCodex, []string{"restart Codex"}},
		{harnessCursor, []string{"new chat"}},
		{harnessGeminiCLI, []string{"/skills reload"}},
		{harnessCopilot, []string{"Copilot CLI: run /skills reload", "VS Code: start a new session"}},
		{harnessOpenCode, []string{"Start a new session."}},
		{harnessAntigravity, []string{"Antigravity CLI 1.2.4 or later", "/skills reload", "new conversation"}},
		{harnessDevin, []string{"Start a new session."}},
	}
	if len(cases) != len(knownHarnesses) {
		t.Fatalf("hint table covers %d harnesses, knownHarnesses has %d", len(cases), len(knownHarnesses))
	}
	for _, tc := range cases {
		t.Run(string(tc.h), func(t *testing.T) {
			t.Parallel()
			hint := loadHint(tc.h)
			if !strings.HasSuffix(hint, fallback) {
				t.Errorf("hint %q does not end with the mandatory fallback sentence", hint)
			}
			for _, w := range tc.wants {
				if !strings.Contains(hint, w) {
					t.Errorf("hint %q does not contain %q", hint, w)
				}
			}
		})
	}
}

func TestHousewordedHintsNameNoCommand(t *testing.T) {
	t.Parallel()

	for _, h := range []harness{harnessOpenCode, harnessDevin} {
		if strings.Contains(loadHint(h), "/") {
			t.Errorf("%s hint %q must not read as a vendor command; the docs are silent", h, loadHint(h))
		}
	}
}
