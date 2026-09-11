//nolint:testpackage // white-box: accesses unexported harness detection symbols
package skill

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o700); err != nil {
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

	// Add .agents → opencode detected.
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o700); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	if !slices.Contains(detectHarnesses(home), harnessOpenCode) {
		t.Fatalf("expected opencode after creating .agents")
	}
}

func TestDetectOrderMatchesKnownHarnesses(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	// Create all footprints.
	for _, dir := range []string{".claude", ".codex", ".gemini", ".copilot", ".cursor", ".agents"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	got := detectHarnesses(home)
	if len(got) != len(knownHarnesses) {
		t.Fatalf("expected %d detected, got %d: %v", len(knownHarnesses), len(got), got)
	}
}
