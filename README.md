# wcloud

The Weaviate Cloud CLI: agent-first provisioning for [Weaviate Cloud](https://console.weaviate.cloud).

`wcloud` creates and inspects Weaviate Cloud clusters from a terminal or from a coding agent.
Every command that returns a result emits a stable JSON envelope on stdout, so an agent can
parse it without screen-scraping, and prints human-readable text when you run it yourself. The
exceptions are `wcloud guide`, which always prints raw Markdown and ignores `--output`, and
cobra's own `--help` text.

Source: <https://github.com/weaviate/weaviate-cloud>

## Status

**Beta (v0.1).** The JSON envelope (`data` / `error` / `metadata`) and the error codes are the
stable part and are meant to be depended on. Commands, flags and text output may still change
between v0.1 releases. There are no tagged releases yet, so building from source is the only
install route today. GitHub Releases, Homebrew, and a version-carrying `go install` all activate
at the first tag.

## Install

### Build from source

Requires **Go 1.26.6 or newer** (the floor is set in `go.mod`).

```
git clone https://github.com/weaviate/weaviate-cloud
cd weaviate-cloud
go build -o wcloud ./cmd/wcloud
```

Works on Linux, macOS and Windows, on amd64 and arm64. On Windows, build `wcloud.exe` and put it
somewhere on your `PATH` yourself; there is no installer. A source build like this one reports
the in-tree default version, `0.1.0-dev`.

### From a tagged release

Once a version is tagged, these channels serve pre-built binaries:

```sh
# go install — carries the real tagged version, no ldflags needed
go install github.com/weaviate/weaviate-cloud/cmd/wcloud@latest

# Homebrew (macOS and Linux)
brew install weaviate/tap/weaviate-cloud

# npm (macOS, Linux and Windows)
npm install -g weaviate-cloud
```

Or download the archive for your platform from the
[GitHub Releases page](https://github.com/weaviate/weaviate-cloud/releases), extract the
`wcloud` binary, and place it on your `$PATH`.

`go install …@latest` prefers a release version but falls back to the newest prerelease when no
release exists yet, and stops serving that prerelease the moment a real release is tagged.
Windows ships only as a `.zip` via GitHub Releases — there is no Homebrew formula for Windows,
and the `.exe` is currently unsigned, so SmartScreen will warn on first run.

The `weaviate-cloud` npm package installs a small shim plus a platform-specific package holding
the real `wcloud` binary, so `npm install` never downloads anything at install time. A final
version (`1.0.0`) publishes to the `latest` dist-tag; a release candidate (`1.0.0-rc1`) publishes
under `next` instead, so `npm install -g weaviate-cloud` never resolves to a prerelease — install
one explicitly with `npm install -g weaviate-cloud@next`.

Check it:

```
wcloud version
```

## Verifying authenticity

Every release's `checksums.txt` is signed with [cosign](https://github.com/sigstore/cosign)
using GitHub Actions keyless signing (Sigstore Fulcio + Rekor). To verify:

```sh
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp "^https://github\.com/weaviate/weaviate-cloud/\.github/workflows/release\.yml@refs/tags/v" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt
```

Expected output: `Verified OK`

## Quick start

```
wcloud auth login                       # sign in through the browser
wcloud region list                      # pick a region
wcloud cluster create --wait            # create a cluster and wait for it
```

`cluster create --wait` blocks until the cluster is ready (up to 15 minutes by default) and
prints the cluster details together with a **one-time API key**. That key is shown once and is
stored nowhere by the CLI, so capture it there and then. Without `--wait`, poll
`wcloud cluster status <id>` until it reports `READY`, then run `wcloud cluster get <id>` for
the key.

The free tier allows one cluster per account and one collection per cluster.

## Authentication

`wcloud` authenticates with OAuth only. There is no API-key flag and no API-key environment
variable.

```
wcloud auth login     # OAuth 2.0 authorization code + PKCE, 127.0.0.1 loopback redirect
wcloud auth whoami    # show the current identity
wcloud auth logout    # revoke and forget the local credentials
```

`auth login` opens your browser and waits up to 5 minutes for the sign-in to complete;
`--timeout` changes the wait and `--no-launch-browser` prints the sign-in URL instead of
opening it. The redirect has to land on `127.0.0.1` on the same machine, so sign-in cannot be
completed from a headless or remote host.

Credentials are cached on disk with `0600` permissions.

## Commands

| Command | What it does |
|---------|--------------|
| `wcloud auth login` | Sign in through the browser and cache credentials |
| `wcloud auth logout` | Revoke and forget the cached credentials |
| `wcloud auth whoami` | Show the identity behind the current session |
| `wcloud cluster create` | Create a cluster (`--name`, `--region`, `--tier`, `--wait`, `--timeout`) |
| `wcloud cluster list` | List the clusters in your organisation |
| `wcloud cluster get <id>` | Fetch one cluster, including the one-time API key on first read |
| `wcloud cluster status <id>` | Fetch a cluster's lifecycle status |
| `wcloud region list` | List available regions |
| `wcloud skill install` | Install the wcloud agent skill into coding harnesses |
| `wcloud guide` | Print the full agent guide as Markdown |
| `wcloud version` | Print version, commit and Go version |

`wcloud login` is an alias for `wcloud auth login`.

Global flags: `--output {auto,json,text}` (`-o`) and `--verbose` (`-v`). The default, `auto`,
prints text when a human is likely to be reading (stdout is a terminal, `CI` is unset and
`TERM` is not `dumb`) and JSON otherwise, so a piped command or a CI job gets the machine
contract. `-o text` and `-o json` override that. Run any command with `--help` for its own
flags.

## Using wcloud with a coding agent

`wcloud guide` prints a full walkthrough written for LLM coding agents: the output contract,
the authentication boundaries, the provisioning lifecycle, and how to handle each failure.
Point your agent at it.

```
wcloud guide
```

`wcloud skill install --all` installs a persistent skill file into every supported harness
(Claude Code, Codex, Cursor, Gemini CLI, Copilot, opencode), so the agent keeps that context
across sessions. Pass `--all` or `--harness <name>`; with neither it prompts, which fails
outside an interactive terminal.

Once a cluster exists, [`weaviate/agent-skills`](https://github.com/weaviate/agent-skills)
gives an agent the data-plane skills to query and populate it.

## Exit codes

| Code | Meaning | Code | Meaning |
|------|---------|------|---------|
| 0 | Success | 5 | Permission denied |
| 1 | Generic error | 6 | Conflict |
| 2 | Usage or validation failed | 7 | Quota exceeded |
| 3 | Authentication required | 8 | Rate limited |
| 4 | Not found | 9 | Service unavailable |

## Reporting problems

- **Bugs and feature requests:** open an issue at
  <https://github.com/weaviate/weaviate-cloud/issues>. Include the output of `wcloud version`
  and, where you can, the failing command re-run with `--verbose -o json`. Redact API keys and
  tokens first.
- **Security vulnerabilities:** do not open a public issue. See [SECURITY.md](./SECURITY.md).
- **Account, billing and cluster problems that are not CLI bugs:**
  <https://console.weaviate.cloud>.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT, see [LICENSE](./LICENSE).
