package skill

import (
	"os"
	"path/filepath"
)

//nolint:gochecknoglobals // immutable ordered mapping; cannot be a const
var footprints = []struct {
	h   harness
	dir string
}{
	{harnessClaudeCode, ".claude"},
	{harnessCodex, ".codex"},
	{harnessCursor, ".cursor"},
	{harnessGeminiCLI, ".gemini"},
	{harnessCopilot, ".copilot"},
	{harnessOpenCode, ".agents"},
}

// detectHarnesses returns the subset of knownHarnesses whose config directory
// is present under home. Order is stable (matches knownHarnesses). No writes.
func detectHarnesses(home string) []harness {
	var found []harness
	for _, fp := range footprints {
		if _, err := os.Stat(filepath.Join(home, fp.dir)); err == nil {
			found = append(found, fp.h)
		}
	}
	return found
}
