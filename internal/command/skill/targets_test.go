//nolint:testpackage // white-box: accesses unexported harness type and target resolution symbols
package skill

import (
	"path/filepath"
	"slices"
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
