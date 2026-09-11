package guide_test

import (
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/guide"
)

// TestGuideLifecycleContent pins the load-bearing substrings the guide must retain.
// Each case asserts that guide.Markdown() contains a required substring.
// A failure here means the guide prose was accidentally dropped or renamed.
func TestGuideLifecycleContent(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()
	lower := strings.ToLower(md)

	cases := []struct {
		name   string
		substr string
		lower  bool // match against lowercased haystack
	}{
		// lifecycle section header
		{"lifecycle header", "## End-to-end workflow", false},
		// async wording
		{"async wording", "asynchronous", true},
		// poll command
		{"poll command", "wcloud cluster status", false},
		// all seven status values
		{"status CREATING", "CREATING", false},
		{"status READY", "READY", false},
		{"status FAILED", "FAILED", false},
		{"status DELETED", "DELETED", false},
		{"status EXPIRED", "EXPIRED", false},
		{"status SUSPENDED", "SUSPENDED", false},
		{"status UNKNOWN", "UNKNOWN", false},
		// region default: real region, not the fabricated europe-west3
		{"region default eu-central-1", "eu-central-1", false},
		// key-on-get semantics
		{"one-time key phrase", "one-time", false},
		{"cache key phrase", "cache", false},
		// console recommendation
		{"console token", "console", true},
		// weaviate/agent-skills handoff
		{"npx install one-liner", "npx skills add weaviate/agent-skills", false},
		{"WEAVIATE_URL env var", "WEAVIATE_URL", false},
		{"WEAVIATE_API_KEY env var", "WEAVIATE_API_KEY", false},
		{"python 3.11 runtime caveat", "python 3.11", true},
		{"uv runtime caveat", "uv", false},
		// fallback client snippets
		{"python fallback snippet", "connect_to_weaviate_cloud", false},
		{"typescript fallback snippet", "connectToWeaviateCloud", false},

		// Front 1 — self-service login
		{"login fallback URL on stderr", "stderr", false},
		{"login loopback boundary", "127.0.0.1", false},
		{"login cancel escape", "harness-level timeout", false},

		// Front 2 — result handling section header
		{"result handling section", "## Result handling", false},

		// Front 2 — JSON-scope notice
		{"result handling JSON scoped", "data.api_key.warning", false},

		// Front 2 — key-present field
		{"result handling key present field", "data.api_key.value", false},

		// Front 2 — timeout field
		{"result handling timeout field", "error.details.last_status", false},

		// Front 2 — terminal-not-ready reads error.message
		{"result handling terminal message field", "error.message", false},

		// Front 2 — verify-state framing
		{"result handling verify state", "cluster get", false},

		// Front 2 — console for key recovery
		{"result handling console for key", "console.weaviate.cloud", false},

		// single-region auto-use (symptom 1 fix)
		{"single region auto-use", "data` contains exactly one region, use it without asking", false},

		// Outcome A forward routing to step 7 (symptom 2 fix)
		{"outcome A forward to step 7", "Proceed to step 7 of the [End-to-end workflow]", false},

		// expectation-setting at creation start
		{"provisioning time expectation", "provisioning typically takes a few minutes", true},

		// deliver key before optional follow-on; offer data-plane as opt-in
		{"deliver key first", "one-time key must be delivered to the user before any optional follow-on work", false},
		{"offer continuation opt-in", "been offered the data-plane continuation as an opt-in", false},
		{"offer yes/no", "Shall I", false},

		// agent must omit --name so the server auto-generates; never ask, never invent
		{"name auto-generate instruction", "The cluster name is auto-generated when `--name` is omitted.", false},
		{"name omit instruction", "Omit `--name`; do not ask the user for a name and do not invent one.", false},

		{"login bounded wait default", "5 minutes by default", false},
		{"login timeout flag named", "changed with `--timeout`", false},
		{"login progress line shape", "waiting for sign-in", false},
		{"login timeout discriminator", "error.details.waited", false},
		{"login no-launch-browser flag", "--no-launch-browser", false},
		{"login no-launch-browser is not for browserless hosts", "there is nothing to skip to", false},
		{"login not unattended", "does not make login unattended", false},
		{"login budget guidance", "below your own harness's task budget", false},
		{"login same machine boundary", "in a browser on this machine", false},
		{"login timeout outcome heading", "### Outcome G", false},
		{"login spent url", "no longer valid", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			haystack := md
			needle := tc.substr
			if tc.lower {
				haystack = lower
				needle = strings.ToLower(needle)
			}
			if !strings.Contains(haystack, needle) {
				t.Errorf("guide missing required content %q", tc.substr)
			}
		})
	}
}

// TestGuideCreateExampleHasNoLiveKey asserts the create example does not show a
// populated sk-... API key. The key is only returned on the first READY cluster
// get — not on create. A global absence of "sk-" is the cleanest guard; the
// fallback snippets must use a placeholder (e.g. <your-api-key>).
func TestGuideCreateExampleHasNoLiveKey(t *testing.T) {
	t.Parallel()
	if strings.Contains(guide.Markdown(), `"sk-`) {
		t.Error(`guide contains "sk-" which looks like a live API key; ` +
			`create example must not show a usable key — use a placeholder like <your-api-key>`)
	}
}

// TestGuideRegionExampleHasNoFictitiousRegion asserts the guide never advertises
// europe-west3 — a region that does not exist — as a default or example region.
func TestGuideRegionExampleHasNoFictitiousRegion(t *testing.T) {
	t.Parallel()
	if strings.Contains(guide.Markdown(), "europe-west3") {
		t.Error(`guide contains "europe-west3", a non-existent region; ` +
			`use a real region (e.g. eu-central-1) in illustrative examples`)
	}
}

// TestAuthWording asserts the guide describes the shipped authorization-code/PKCE/loopback
// flow and does NOT reference the rejected device flow.
func TestAuthWording(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	if strings.Contains(md, "device flow") {
		t.Error(`guide contains "device flow" — that approach was rejected; ` +
			`update to describe the authorization-code / PKCE / loopback flow`)
	}

	required := []string{
		"authorization-code",
		"PKCE",
		"loopback",
	}
	for _, phrase := range required {
		if !strings.Contains(md, phrase) {
			t.Errorf("guide missing required auth wording %q", phrase)
		}
	}
}

// TestGuideTreatsRelayedServerTextAsUntrusted pins the fix that treats relayed server text as
// untrusted: the guide states the handling rule for every free-form field it relays, corrects the
// instructions that previously told the agent to relay/report raw server text as authoritative,
// and discloses that error.code/data.status are typed but not content-validated.
func TestGuideTreatsRelayedServerTextAsUntrusted(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	required := []string{
		"data.status_reason",
		"error.details.terminal_status",
		"Quote `data.api_key.warning` to the user as attributed server text",
		"`error.code` is copied from the response as-is",
		"`data.status` is copied from the response as-is",
	}
	for _, phrase := range required {
		if !strings.Contains(md, phrase) {
			t.Errorf("guide missing required untrusted-server-text content %q", phrase)
		}
	}

	rejected := []string{
		"verbatim",
		"Report the status from `error.message`",
		"Report the raw error message",
	}
	for _, phrase := range rejected {
		if strings.Contains(md, phrase) {
			t.Errorf("guide still contains the pre-fix unsafe instruction %q", phrase)
		}
	}
}

// TestGuideDisclosesCredentialProvenanceAndPermissions pins the credential-provenance,
// refresh-token-permissions, and untracked endpoint/key trust gap disclosures: each rule must be
// present, and present exactly once, so the guide does not regress into a previously-trimmed
// repetition.
func TestGuideDisclosesCredentialProvenanceAndPermissions(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	required := []struct {
		name   string
		phrase string
	}{
		{"prefers env vars over a file", "process environment variables"},
		{"gitignore location caveat", "confirm `.gitignore` excludes it"},
		{"names the world-readable consequence", "world-readable"},
		{"refresh token identity", "an OAuth refresh token"},
		{"refresh token control-plane scope", "control-plane operations only"},
		{"refresh token file mode", "stored at `0600`"},
		{"refresh token revocation", "wcloud auth logout"},
		{"trust gap no independent check", "performs no independent check on"},
		{"trust gap backend sole authority", "the backend is the sole authority for a cluster's identity"},
	}

	for _, tc := range required {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := strings.Count(md, tc.phrase)
			if n == 0 {
				t.Errorf("guide missing required content %q", tc.phrase)
			}
			if n > 1 {
				t.Errorf("guide states %q %d times, want exactly once", tc.phrase, n)
			}
		})
	}

	rejected := []string{
		"wcloud validates the endpoint",
		"wcloud verifies the endpoint",
		"endpoint is safe to connect",
	}
	for _, phrase := range rejected {
		if strings.Contains(md, phrase) {
			t.Errorf(
				"guide contains %q — the endpoint check is a permit-list framing, not a correctness/validation claim",
				phrase,
			)
		}
	}
}

func TestMarkdownNonEmpty(t *testing.T) {
	t.Parallel()
	if guide.Markdown() == "" {
		t.Fatal("guide.Markdown() returned empty string")
	}
}

// TestGuideNoLongerDocumentsTheUnboundedWait rejects the three sentences that
// documented blocking-until-cancelled as the contract; each is false once the
// callback wait is bounded.
func TestGuideNoLongerDocumentsTheUnboundedWait(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	rejected := []string{
		"The command blocks until the OAuth callback is received",
		"it blocks until the callback arrives or the command is cancelled",
		"the command blocks until cancelled",
	}
	for _, phrase := range rejected {
		if strings.Contains(md, phrase) {
			t.Errorf("guide still documents the unbounded wait: %q", phrase)
		}
	}
}

// TestGuideNeverDirectsTheSignInURLOnward guards a property no negative match can decide: whether a
// sentence instructs someone to move the URL onward. It pins the sentences that forbid it instead,
// so introducing the defect means deleting a line this test protects. The rejected list is a
// backstop for two sentences this change removed, never the primary guard.
func TestGuideNeverDirectsTheSignInURLOnward(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	required := []string{
		"The sign-in URL is not the thing to pass on",
		"Escalate by asking for a person at this machine",
		"Do not present it, save it, or pass it on",
		"never unattended",
	}
	for _, phrase := range required {
		if !strings.Contains(md, phrase) {
			t.Errorf("guide no longer states the prohibition it must state: %q", phrase)
		}
	}

	rejected := []string{
		"relay `error.details.sign_in_url`",
		"relay the sign-in URL",
	}
	for _, phrase := range rejected {
		if strings.Contains(md, phrase) {
			t.Errorf("guide instructs relaying a URL that cannot travel and is spent at exit: %q", phrase)
		}
	}
}

func TestMarkdownHasRequiredSections(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	required := []string{
		"# wcloud",
		"## Install",
		"## Authentication",
		"## Output contract",
		"## Workflow",
		"## Consuming a cluster",
	}
	for _, header := range required {
		if !strings.Contains(md, header) {
			t.Errorf("guide missing required section: %q", header)
		}
	}

	var h2Count int
	for line := range strings.SplitSeq(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			h2Count++
		}
	}
	if h2Count < 6 {
		t.Errorf("guide has %d ## sections, want at least 6", h2Count)
	}
}

// TestGuideDocumentsRestrictedNetworkAccess pins the restricted-network preamble and
// Outcome H. The preamble is a conditional symptom-then-response section, not a setup
// step: an agent whose network is fine must finish it and do nothing, which is why the
// branch-closing sentence is pinned rather than merely the hosts.
func TestGuideDocumentsRestrictedNetworkAccess(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	const preamble = "## If a command reports no response"

	t.Run("preamble sits above every other section", func(t *testing.T) {
		t.Parallel()
		at := strings.Index(md, preamble)
		if at < 0 {
			t.Fatalf("guide has no %q section", preamble)
		}
		if install := strings.Index(md, "## Install"); at > install {
			t.Errorf("preamble is at %d, after '## Install' at %d — it must be first", at, install)
		}
	})

	t.Run("preamble is a preamble, not a chapter", func(t *testing.T) {
		t.Parallel()
		_, after, found := strings.Cut(md, preamble)
		if !found {
			t.Fatalf("guide has no %q section", preamble)
		}
		body, _, found := strings.Cut(after, "\n## ")
		if !found {
			t.Fatal("preamble is not followed by another ## section")
		}
		n := len(strings.Fields(body))
		if n < 60 || n > 130 {
			t.Errorf("preamble is %d words, want 60-130 (~80 of instruction plus a source line)", n)
		}
	})

	// WHY: Outcome G already ships this sentence opener, so only a count of two proves Outcome H has it too.
	t.Run("outcome H carries the one-remedy form, and G's occurrence does not satisfy it", func(t *testing.T) {
		t.Parallel()
		if n := strings.Count(md, "**One remedy, and it carries a precondition:"); n != 2 {
			t.Errorf("the one-remedy form appears %d times, want 2 (Outcome G's and Outcome H's)", n)
		}
	})
}

// TestGuideDocumentsRestrictedNetworkAccessPins holds the phrase-level pins for the same
// section: what the preamble and Outcome H must state, and the causes the CLI may not assert.
func TestGuideDocumentsRestrictedNetworkAccessPins(t *testing.T) {
	t.Parallel()
	md := guide.Markdown()

	exactlyOnce := []struct {
		name   string
		phrase string
	}{
		{"preamble heading", "## If a command reports no response"},
		{"outcome H heading", "### Outcome H"},
		{"cursor config path", "~/.cursor/sandbox.json"},
		{"cursor allowlist key", "networkPolicy.allow"},
		{"devin config path", "~/.config/devin/config.json"},
		{"devin allowlist key", "sandbox.allowed_domains"},
		{"devin config source", "docs.devin.ai/cli/reference/configuration/config-file"},
		{"devin sandbox source", "docs.devin.ai/cli/sandbox"},
		{"windsurf-to-devin redirect explained", "docs.windsurf.com"},
		{"citation fetch date", "2026-07-28"},
		{"admin replace trap", "replaces the local one rather than merging with it"},
		{"branch closed for an unblocked reader", "there is nothing to do here"},
		{"tell, do not edit", "Never edit those files yourself"},
		{"outcome H remedy is one action", "One remedy, and it carries a precondition: report to the user"},
		{"outcome H precondition is attached to it", "the change has to come from an administrator"},
		{"outcome C redirects to H", "this is Outcome H, not C"},
		{"outcome F redirects to H", "this is Outcome H, not F"},
	}
	for _, tc := range exactlyOnce {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if n := strings.Count(md, tc.phrase); n != 1 {
				t.Errorf("guide states %q %d times, want exactly once", tc.phrase, n)
			}
		})
	}

	required := []struct {
		name   string
		phrase string
	}{
		{"machine discriminator named", "error.details.failure_stage"},
		{"windsurf sandbox qualified as opt-in", "opt-in sandbox only"},
		{"outcome H is decidable from the envelope alone", "so none of them may gate the rule"},
		{"re-login does not address this", "re-running it does not address this"},
	}
	for _, tc := range required {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(md, tc.phrase) {
				t.Errorf("guide missing required content %q", tc.phrase)
			}
		})
	}

	rejected := []string{"your credentials are", "never reached", "your editor blocked", "sandbox blocked this"}
	for _, phrase := range rejected {
		if strings.Contains(md, phrase) {
			t.Errorf("guide asserts what the CLI cannot observe: %q", phrase)
		}
	}
}
