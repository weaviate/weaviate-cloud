//nolint:testpackage // white-box: accesses renderSKILL() which is unexported
package skill

import (
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/guide"
)

func TestRenderSkill(t *testing.T) {
	t.Parallel()
	got := renderSKILL()

	// Must start with frontmatter delimiter.
	if !strings.HasPrefix(got, "---\n") {
		limit := min(20, len(got))
		t.Fatalf("SKILL.md does not start with ---\\n, got prefix %q", got[:limit])
	}

	// Must contain name: wcloud.
	if !strings.Contains(got, "name: wcloud") {
		t.Errorf("SKILL.md missing 'name: wcloud'")
	}

	// Description must be present and contain required keywords.
	if !strings.Contains(got, "description:") {
		t.Fatalf("SKILL.md missing 'description:' line")
	}
	descLine := findDescriptionLine(got)
	if !strings.Contains(descLine, "Weaviate Cloud") {
		t.Errorf("description does not contain 'Weaviate Cloud': %q", descLine)
	}
	if !strings.Contains(descLine, "wcloud") {
		t.Errorf("description does not contain 'wcloud': %q", descLine)
	}
	descValue := strings.TrimPrefix(descLine, "description:")
	descValue = strings.TrimSpace(strings.Trim(descValue, "\""))
	if len(descValue) == 0 || len(descValue) > 1024 {
		t.Errorf("description length = %d, want 1..1024", len(descValue))
	}

	// Frontmatter must be closed.
	if !strings.Contains(got, "\n---\n") {
		t.Fatalf("SKILL.md has no closing --- frontmatter delimiter")
	}

	// Body after closing --- must equal guide.Markdown() exactly (single-source guarantee).
	parts := strings.SplitN(got, "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("could not split SKILL.md at closing ---")
	}
	// The body follows the closing delimiter; strip the leading newline that separates
	// frontmatter from body.
	body := strings.TrimPrefix(parts[1], "\n")
	want := guide.Markdown()
	if body != want {
		t.Errorf("SKILL.md body differs from guide.Markdown()\ngot  len=%d\nwant len=%d", len(body), len(want))
	}
}

func findDescriptionLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(line, "description:") {
			return line
		}
	}
	return ""
}
