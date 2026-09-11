package skill

import (
	"strings"

	"github.com/weaviate/weaviate-cloud/internal/guide"
)

const (
	skillName = "wcloud"
	// skillDescription is the razor-sharp trigger string surfaced to agents.
	// It names the exact conditions under which the skill should activate.
	skillDescription = "Provision, list, inspect, and manage Weaviate Cloud clusters with the wcloud CLI. " +
		"Use when the user wants to create, list, get status of, or consume a Weaviate Cloud cluster, " +
		"authenticate to Weaviate Cloud, or drive wcloud from an agent."
)

// renderSKILL produces the full SKILL.md content: YAML frontmatter followed
// by the canonical agent guide verbatim. This is the single-source guarantee:
// the body bytes are always equal to guide.Markdown().
func renderSKILL() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(skillName)
	b.WriteString("\n")
	b.WriteString("description: \"")
	b.WriteString(skillDescription)
	b.WriteString("\"\n")
	b.WriteString("---\n\n")
	b.WriteString(guide.Markdown())
	return b.String()
}
