# Architecture Rules

## Layout

```
cmd/wcloud/main.go                   wiring + error-envelope writer + os.Exit
internal/
  cli/                               root cobra cmd + global flags + entry-level subcommands (version, guide)
  factory/                           shared CLI deps (IOStreams, Now, NewRequestID, NewAPIClient, Auth, OutputFormat)
  iostreams/                         stdin/stdout/stderr + TTY detection (build-tagged per platform)
  api/                               hand-rolled HTTP client + duplicated wire types per resource
  auth/                              OAuth authorization code + PKCE, loopback callback + token cache (filesystem)
  allowlist/                         credential-destination policy: may this token be sent to this URL
  config/                            profiles.json parser + env-var resolver (precedence: env > active profile > default)
  output/                            JSON envelope writer + Table/KeyValue text renderers + Write/WriteEnv dispatch
  errcode/                           frozen error codes → integer exit code mapping
  guide/                             guide.md + go:embed accessor, printed by `wcloud guide`
  cmdtest/                           shared test harness for driving the root command end to end
  command/                           flyctl-style noun directories, verb files
    auth/{auth,login,logout,whoami,render}.go
    cluster/{cluster,list,get,status,create,await,render}.go
    region/{region,list,render}.go
    skill/{skill,install,detect,detect_agent,targets,render}.go
    profile/{profile,list,create,use,delete,show,render,wizard,metadata}.go   (Hidden: true — internal env switching)
```

`internal/cmdtest/` is a test-only package: it imports `cli`, `auth` and `config` to build a
fully wired root command, so no other package may import it. `internal/allowlist/` carries
build-tagged files (`loopback_dev.go` / `loopback_release.go`) that gate loopback destinations
on the `wcloud_dev` build tag, and a `NewPermissiveForTesting` constructor whose non-test use is
blocked by a `forbidigo` rule in `.golangci.yml`.

## Architecture pattern: flyctl-style, NOT hexagonal

`wcloud` follows the `superfly/flyctl` + `cli/cli` + `stripe/stripe-cli` blend: a Factory-based dependency-injection chassis (`internal/factory/`) carries shared dependencies into each cobra command. Each noun is a directory (`internal/command/cluster/`); each verb is a file (`list.go`).

**Do not introduce ports-and-adapters here.** A service with many swappable I/O adapters earns a hexagonal, ports-and-adapters layer; a CLI with one HTTP client, one auth provider, and one filesystem cache does not. The cost of an extra interface layer would be higher than its testability benefit at this scale.

Inspiration sources, in priority order: `superfly/flyctl` for directory layout (`internal/command/{noun}/{verb}.go`); `cli/cli` for the Factory DI carrier and `IOStreams` abstraction; `stripe/stripe-cli` for stable JSON output discipline.

## Dependency rules

- `cmd/wcloud/main.go` → wires factory + cli; no business logic.
- `internal/cli/` → depends on `factory`, `output`, `errcode`. Owns the root cobra tree.
- `internal/factory/` → constructs shared deps; depends on `iostreams`, and later `api`, `auth`, `config`.
- `internal/command/<noun>/` → depends on `factory`, `output`, and `errcode`. Commands consume `f.APIClient` / `f.Auth` through the Factory; they never instantiate clients directly.
- `internal/api/` → pure HTTP client. No awareness of cobra, no awareness of output formatting.
- `internal/auth/` → owns the authorization-code + PKCE flow, the loopback callback listener, and the credential file. No cobra imports. RFC 8628 device flow was considered and rejected; do not describe this package as implementing it.
- `internal/allowlist/` → leaf package. Depends on nothing in this module; `api` and `auth` consume it.
- `internal/output/`, `internal/errcode/`, `internal/guide/` → leaf packages. No imports from `internal/command/*`.

**Cross-noun imports are the exception, not the rule.** Commands share state through the Factory; a `command/<noun>/` package does not import another to reach its command constructors, its flag types, or its rendering. The one sanctioned exception in the tree today is `cluster/create.go` importing `command/skill` for `skill.IsAgentDriven()`, a pure environment probe that carries no cobra or Factory state. Before adding another, check whether the thing being reused is really a leaf helper; if it is, move it to a leaf package rather than importing a noun.

## Adding a new command

1. Identify the noun (`auth`, `cluster`, `region`, `skill`, `profile`). Create the noun directory if it doesn't exist, with a `noun.go` registering the cobra group.
2. Add a verb file: `internal/command/<noun>/<verb>.go` exporting `NewXxxCmd(f *factory.Factory) *cobra.Command`.
3. If the verb needs a new API call, add the method to `internal/api/<noun>.go`. Hand-write the request/response types in `internal/api/types.go` matching the backend API's public v1 surface.
4. Register the new cobra command from its noun's `noun.go`.
5. Cover the verb with a test that drives `cobra.Command.Execute()` with a mocked `*factory.Factory`.

## Output and errors

Every command returns either a typed envelope (`output.Envelope[T]`) on success or an `*errcode.Error` on failure. The one exception is `wcloud guide`, which writes raw Markdown to stdout and ignores `--output` entirely; cobra's own help/usage text is likewise unenveloped. The root command's error handler renders the error envelope and `os.Exit`s with the mapped integer exit code. Commands never call `os.Exit` directly. Human-readable text (auth prompts, `--verbose` logs) goes to stderr. In text format (`--output text`, or `auto` resolving to text), commands render `output.Table`/`output.KeyValue` to stdout instead of the JSON envelope; the format is resolved once into `f.OutputFormat` via the root `PersistentPreRunE`. `auto` resolves to text only when stdout is a TTY **and** `CI` is empty **and** `TERM` is not `dumb`; a PTY alone does not imply a human reader. Any subcommand that defines its own `PreRunE`/`PersistentPreRunE` must call the root's persistent hook explicitly (or rely on `RunE`), because cobra does NOT chain persistent pre-runs — otherwise `f.OutputFormat` won't be set.

## Configuration

v0.1 resolves config per-field with precedence **env var > active profile > built-in production default**. `internal/config/` parses and validates a `profiles.json` config file stored in `os.UserConfigDir()/wcloud/`; the env vars `WCLOUD_ENDPOINT`, `WCLOUD_AUTH_BASE_URL`, and `WCLOUD_AUTH_CLIENT_ID` override both the active profile and the compiled-in defaults.

An unparseable `profiles.json` fails the command with `validation_failed` (exit 2) and the file path named, rather than silently falling back to production defaults. `cmd/wcloud/main.go` exempts `guide` and `version` so an agent can still read how to fix it (`needsConfig`).

The `wcloud profile` command group (`Hidden: true`) is an internal surface for Weaviate engineering to point the CLI at non-production environments without baking environment names into the public binary. The active profile also scopes the on-disk credentials file. There is no `--profile` global flag — the active profile is selected via `wcloud profile use`.

A public, user-facing multi-org account/profile system (post-v0.2) would build on this resolver; the current surface is deliberately hidden, not user-facing.
