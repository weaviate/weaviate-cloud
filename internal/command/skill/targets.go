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

	harnessAntigravity harness = "antigravity"
	harnessDevin       harness = "devin"
)

const (
	harnessAliasWindsurf = "windsurf"
	geminiDir            = ".gemini"
)

//nolint:gochecknoglobals // immutable ordered slice; cannot be a const
var knownHarnesses = []harness{
	harnessClaudeCode,
	harnessCodex,
	harnessCursor,
	harnessGeminiCLI,
	harnessCopilot,
	harnessOpenCode,
	harnessAntigravity,
	harnessDevin,
}

// targetPath returns the absolute SKILL.md path for the given harness.
// Claude Code uses its native ~/.claude/skills/ path. Antigravity CLI does not
// document reading ~/.agents/skills/, so at user scope it gets its own
// ~/.gemini/antigravity-cli/skills/; at project scope it reads .agents/skills/
// like the rest. All others use the cross-vendor .agents/skills/ alias that
// Codex, Cursor, Copilot, OpenCode, Gemini CLI and Devin all honour.
func targetPath(h harness, home, projectRoot string, project bool) string {
	root := home
	if project {
		root = projectRoot
	}
	var dir string
	switch {
	case h == harnessClaudeCode:
		dir = filepath.Join(root, ".claude", "skills", "wcloud")
	case h == harnessAntigravity && !project:
		dir = filepath.Join(root, geminiDir, "antigravity-cli", "skills", "wcloud")
	default:
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

func harnessNames() []string {
	names := make([]string, len(knownHarnesses))
	for i, h := range knownHarnesses {
		names[i] = string(h)
		if h == harnessDevin {
			names[i] += " (alias: " + harnessAliasWindsurf + ")"
		}
	}
	return names
}
