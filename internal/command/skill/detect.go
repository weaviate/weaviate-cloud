package skill

import (
	"os"
	"path/filepath"
	"slices"
)

//nolint:gochecknoglobals // immutable ordered mapping; cannot be a const
var footprints = []struct {
	h    harness
	dirs []string
}{
	{harnessClaudeCode, []string{".claude"}},
	{harnessCodex, []string{".codex"}},
	{harnessCursor, []string{".cursor"}},
	{harnessGeminiCLI, []string{".gemini"}},
	// WHY: VS Code runs Copilot and reads the shared skills folder; .vscode is its per-user folder on all three OSes.
	{harnessCopilot, []string{".copilot", ".vscode"}},
	{harnessOpenCode, []string{filepath.Join(".config", "opencode")}},
}

// detectHarnesses returns the subset of knownHarnesses with at least one config
// directory present under home. Order is stable (matches knownHarnesses). No writes.
func detectHarnesses(home string) []harness {
	var found []harness
	for _, fp := range footprints {
		present := slices.ContainsFunc(fp.dirs, func(dir string) bool {
			_, err := os.Stat(filepath.Join(home, dir))
			return err == nil
		})
		if present {
			found = append(found, fp.h)
		}
	}
	return found
}
