//nolint:testpackage // white-box: accesses unexported isEnvTruthy
package skill

import "testing"

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestIsAgentDriven_NoVars(t *testing.T) {
	for _, key := range []string{
		"CLAUDECODE", "CLAUDE_CODE", "CURSOR_AGENT", "GEMINI_CLI",
		"AGENT",
		"CODEX", "OPENAI_CODEX", "AIDER", "CLINE", "WINDSURF_AGENT",
		"GITHUB_COPILOT", "AMAZON_Q", "AWS_Q_DEVELOPER", "GEMINI_CODE_ASSIST",
		"OPENCODE", "SRC_CODY", "PI_CODING_AGENT", "FORCE_AGENT_MODE",
	} {
		t.Setenv(key, "")
	}
	if IsAgentDriven() {
		t.Fatal("expected false with no agent env vars set")
	}
}

func TestIsAgentDriven_CLAUDECODE(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	if !IsAgentDriven() {
		t.Fatal("expected true with CLAUDECODE=1")
	}
}

func TestIsAgentDriven_FORCE_AGENT_MODE(t *testing.T) {
	t.Setenv("FORCE_AGENT_MODE", "1")
	if !IsAgentDriven() {
		t.Fatal("expected true with FORCE_AGENT_MODE=1")
	}
}

func TestIsAgentDriven_GEMINI_CLI(t *testing.T) {
	t.Setenv("GEMINI_CLI", "1")
	if !IsAgentDriven() {
		t.Fatal("expected true with GEMINI_CLI=1")
	}
}

func TestIsAgentDriven_AGENT_NonEmpty(t *testing.T) {
	t.Setenv("AGENT", "goose")
	if !IsAgentDriven() {
		t.Fatal("expected true with AGENT=goose")
	}
}

func TestIsAgentDriven_AGENT_Empty(t *testing.T) {
	for _, key := range []string{
		"CLAUDECODE", "CLAUDE_CODE", "CURSOR_AGENT", "GEMINI_CLI",
		"CODEX", "OPENAI_CODEX", "AIDER", "CLINE", "WINDSURF_AGENT",
		"GITHUB_COPILOT", "AMAZON_Q", "AWS_Q_DEVELOPER", "GEMINI_CODE_ASSIST",
		"OPENCODE", "SRC_CODY", "PI_CODING_AGENT", "FORCE_AGENT_MODE",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("AGENT", "")
	if IsAgentDriven() {
		t.Fatal("expected false with AGENT='' and no other agent vars")
	}
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestIsEnvTruthy(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"", false},
		{"false", false},
		{"yes", false},
		{"0", false},
	}
	for _, tc := range cases {
		t.Setenv("FORCE_AGENT_MODE", tc.value)
		got := isEnvTruthy("FORCE_AGENT_MODE")
		if got != tc.want {
			t.Errorf("isEnvTruthy(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
