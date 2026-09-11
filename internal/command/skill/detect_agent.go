package skill

import (
	"os"
	"slices"
	"strings"
)

const (
	envClaudeCode       = "CLAUDECODE"
	envClaudeCodeAlias  = "CLAUDE_CODE"
	envCursorAgent      = "CURSOR_AGENT"
	envGeminiCLI        = "GEMINI_CLI"
	envCodex            = "CODEX"
	envOpenAICodex      = "OPENAI_CODEX"
	envAider            = "AIDER"
	envCline            = "CLINE"
	envWindsurfAgent    = "WINDSURF_AGENT"
	envGitHubCopilot    = "GITHUB_COPILOT"
	envAmazonQ          = "AMAZON_Q"
	envAWSQDeveloper    = "AWS_Q_DEVELOPER"
	envGeminiCodeAssist = "GEMINI_CODE_ASSIST"
	envOpenCode         = "OPENCODE"
	envSrcCody          = "SRC_CODY"
	envPICodingAgent    = "PI_CODING_AGENT"
	envForceAgentMode   = "FORCE_AGENT_MODE"
)

// IsAgentDriven reports whether the environment signals a coding-agent harness is executing this CLI.
// Detects only known harness env vars; an unrecognised harness falls back to cli.
func IsAgentDriven() bool {
	if slices.ContainsFunc([]string{
		envClaudeCode,
		envClaudeCodeAlias,
		envCursorAgent,
		envGeminiCLI,
		envCodex, envOpenAICodex,
		envAider,
		envCline,
		envWindsurfAgent,
		envGitHubCopilot,
		envAmazonQ, envAWSQDeveloper,
		envGeminiCodeAssist,
		envOpenCode,
		envSrcCody,
		envPICodingAgent,
		envForceAgentMode,
	}, isEnvTruthy) {
		return true
	}
	return os.Getenv("AGENT") != ""
}

func isEnvTruthy(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "1" || v == "true"
}
