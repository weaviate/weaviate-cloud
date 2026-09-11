package skill

import "path/filepath"

type harness string

const (
	harnessClaudeCode harness = "claude-code"
	harnessCodex      harness = "codex"
	harnessCursor     harness = "cursor"
	harnessGeminiCLI  harness = "gemini-cli"
	harnessCopilot    harness = "copilot"
	harnessOpenCode   harness = "opencode"
)

//nolint:gochecknoglobals // immutable ordered slice; cannot be a const
var knownHarnesses = []harness{
	harnessClaudeCode,
	harnessCodex,
	harnessCursor,
	harnessGeminiCLI,
	harnessCopilot,
	harnessOpenCode,
}

// targetPath returns the absolute SKILL.md path for the given harness.
// Claude Code uses its native ~/.claude/skills/ path; all others use the
// cross-vendor ~/.agents/skills/ alias that Codex, Cursor, Copilot,
// OpenCode, and Gemini CLI all honour.
func targetPath(h harness, home, projectRoot string, project bool) string {
	root := home
	if project {
		root = projectRoot
	}
	var dir string
	if h == harnessClaudeCode {
		dir = filepath.Join(root, ".claude", "skills", "wcloud")
	} else {
		dir = filepath.Join(root, ".agents", "skills", "wcloud")
	}
	return filepath.Join(dir, "SKILL.md")
}

// resolveTargets maps the selected harnesses to write paths, deduplicating
// paths that are identical (e.g. codex + cursor both map to .agents/).
func resolveTargets(harnesses []harness, home, projectRoot string, project bool) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, h := range harnesses {
		p := targetPath(h, home, projectRoot, project)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths
}
