# CLAUDE.md — weaviate-cloud

Project notes for Claude Code working in this repo. Keep additions specific and verifiable. If a rule would not change Claude's behavior, don't add it.

## Project

`wcloud` is the Weaviate Cloud CLI — agent-first provisioning over the Weaviate Cloud backend API.

- Module: `github.com/weaviate/weaviate-cloud`
- Binary: `wcloud`
- License: MIT, public OSS

Architecture: thin **hand-rolled** HTTP client. The CLI duplicates wire types from the backend by hand — there is no OpenAPI codegen for v0.1. Upstream additive backend changes are silent; see `CONTRIBUTING.md` for the maintenance discipline.

## Toolchain

Tool versions are pinned in `mise.toml` (Go 1.26.6, golangci-lint 2.12.2, mockery 2.53.6). `go.mod` sets the module's Go floor at 1.26.6. Don't change them ad-hoc — bump them in `mise.toml` so CI and local stay aligned. Prefix commands with `mise exec --` if mise is not activated in your shell.

```bash
go build ./...                          # build
go vet ./...                            # vet
go test ./...                           # all tests
go test -race ./...                     # what CI runs
go test ./internal/auth/...             # one package
golangci-lint run --timeout=5m          # lint
make build | lint | test | tidy | mocks # Makefile shortcuts
```

`.github/workflows/ci.yml` runs on every push to `main` and every pull request, in four jobs: **Lint** (`golangci-lint run --timeout=5m`), **Build** (a matrix running `go vet ./...` then `go build ./...` for linux, darwin and windows on amd64 and arm64), **Test** (`go test -race ./...`, linux/amd64 only), and **Vulnerability scan** (`govulncheck -mode=binary` over the built binary; binary mode because govulncheck's source mode cannot parse a Go 1.26 module today). Lint must be clean before opening a PR — fix the code, don't `//nolint` it (and never `//nolint` without an inline reason).

Test execution is linux-only; the other platforms are compile-and-vet coverage. If you touch a build-tagged file (`internal/iostreams/tty_*.go`, `internal/allowlist/loopback_*.go`), CI will compile it but will not run it.

@.claude/rules/architecture.md
@.claude/rules/git.md
@.claude/rules/testing.md

## Code style

These override defaults — anything not listed follows standard Go conventions.

- **Comments**: default to none — well-named identifiers do the job. Don't over-explain or narrate. Add a comment only when there's a credible necessity: a weird edge case, a `TODO`/`FIXME` with context, a workaround for a real bug, a hidden constraint, or genuinely complex logic that won't make sense to the next reader without a line of explanation. Never restate what the code does or reference the task that introduced it.
- **No premature abstractions**: three similar lines beats a wrong abstraction. Wait for a real third caller before extracting.
- **No defensive code for impossible cases**: trust internal callers and framework guarantees. Validate only at system boundaries (user input, network responses).
- **Error wrapping**: `fmt.Errorf("…: %w", err)` for context. Don't invent fallbacks or recovery paths for errors that should propagate.
- **Imports**: grouped `standard | default | github.com/weaviate/weaviate-cloud` (enforced by `.golangci.yml`).

## Architectural pins (locked — do not relitigate)

Decisions already made. Don't propose alternatives without explicit user direction.

- **No OpenAPI codegen.** `internal/api/` stays hand-rolled. Don't add `oapi-codegen`, `go:generate`, or auto-sync tooling.
- **OAuth-only auth.** No `--token` flag, no `WEAVIATE_API_KEY`. Ever.
- **No viper.** Config is hand-rolled.
- **v0.1 global flag surface is exactly `--output`, `--verbose`** (plus cobra's `--help` / `--version`). `--endpoint`, `--api-version`, `--no-color`, `--yes` are deferred — do not wire them "for forward compatibility". `--quiet` was removed after shipping inert (parsed, never read anywhere) — don't reintroduce it without a specified suppression policy. There is no `--profile` *flag*; profile selection happens through the hidden `wcloud profile` command group (next pin), not a global flag.
- **The profile system is shipped in v0.1, and it is intentional.** `internal/config/` parses and validates a `profiles.json` config file (in `os.UserConfigDir()/wcloud/`), and the hidden (`Hidden: true`) `wcloud profile` command group (`list/create/use/delete/show`) lets Weaviate engineering point the CLI at non-production environments without baking environment names into the public binary. Config resolves per-field with precedence **env var > active profile > built-in default** (`WCLOUD_ENDPOINT`, `WCLOUD_AUTH_BASE_URL`, `WCLOUD_AUTH_CLIENT_ID`); profiles also scope the on-disk credentials file. This deliberately supersedes the earlier "no config file / thin resolver" stance — do not remove or relitigate it.
- **Dual-format output.** Default (`auto`) is human-readable plain text only when stdout is a terminal **and** `CI` is empty **and** `TERM` is not `dumb`; JSON in every other case, including piped/redirected stdout and CI runners that allocate a PTY. `--output {auto,json,text}` overrides. JSON is the stable machine contract (envelope unchanged). Text rendering uses `output.Table`/`output.KeyValue`; API types stay free of formatting knowledge. No color. `--verbose` logs and status prompts still go to stderr.

## Working style

How Claude should behave in this repo regardless of how the user phrases a request.

- **Push back when warranted.** If a request conflicts with an architectural pin, contradicts existing code, or looks like a likely mistake, say so before implementing. A confident "yes, here you go" that ships a regression is worse than five seconds of clarification.
- **Don't fake agreement.** If you need to think, think. If the user's premise looks wrong, challenge it with evidence.
- **Ask when blocked.** Real-money decisions, scope choices, naming, and irreversible operations are user calls — use `AskUserQuestion` rather than guessing.
- **Don't fabricate.** If you're unsure whether a function, flag, or library API exists, look it up — grep the codebase, read the docs, run `--help`. Hallucinated APIs are the most common failure mode.
- **Research before declaring.** For broad questions where neither you nor the user has priors, fetch authoritative sources before answering. Don't pattern-match from training data on language/tooling versions.
- **One change per intent.** Don't bundle a refactor with a bug fix. A PR description should fit in one sentence.

## Repo etiquette

Git, branch, and PR conventions are in `@.claude/rules/git.md` (imported above). One repo-specific addendum:

- Don't override the committer identity with `-c user.email=…`. Use the global git config.
