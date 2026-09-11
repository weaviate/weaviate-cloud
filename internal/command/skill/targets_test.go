//nolint:testpackage // white-box: accesses unexported harness type and target resolution symbols
package skill

import (
	"path/filepath"
	"testing"
)

func TestKnownHarnesses(t *testing.T) {
	t.Parallel()
	want := map[harness]bool{
		harnessClaudeCode: true,
		harnessCodex:      true,
		harnessCursor:     true,
		harnessGeminiCLI:  true,
		harnessCopilot:    true,
		harnessOpenCode:   true,
	}
	if len(knownHarnesses) != len(want) {
		t.Fatalf("len(knownHarnesses) = %d, want %d", len(knownHarnesses), len(want))
	}
	for _, h := range knownHarnesses {
		if !want[h] {
			t.Errorf("unexpected harness %q in knownHarnesses", h)
		}
	}
	// windsurf must not appear (path unverified, deferred per Brief decision 4).
	for _, h := range knownHarnesses {
		if h == "windsurf" {
			t.Error("windsurf must not be in knownHarnesses (path unverified, deferred)")
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
