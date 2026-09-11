# Contributing

Thanks for your interest in contributing to `wcloud`.

Bug reports and feature requests belong in
[GitHub issues](https://github.com/weaviate/weaviate-cloud/issues). Security vulnerabilities do
not: see [SECURITY.md](./SECURITY.md).

## Getting set up

Tool versions are pinned in `mise.toml`, so [mise](https://mise.jdx.dev) is the shortest route
to a matching toolchain:

```
mise install
```

That gives you Go 1.26.6, golangci-lint 2.12.2 and mockery 2.53.6, the same versions CI uses.
Without mise, install those versions yourself; `go.mod` sets the Go floor at 1.26.6 and an
older toolchain will not build the module.

If mise is installed but not activated in your shell, prefix commands with `mise exec --`.

## Build, test, lint

The `Makefile` wraps the four commands you need:

```
make build     # go build ./...
make test      # go test ./...
make lint      # golangci-lint run
make tidy      # go mod tidy
make mocks     # regenerate mockery mocks from .mockery.yaml
```

Run the underlying commands directly when you want narrower scope:

```
go test ./internal/auth/...          # one package
go test -race ./...                  # what CI runs
go vet ./...
golangci-lint run --timeout=5m
```

Build and run the binary itself:

```
go build -o wcloud ./cmd/wcloud
./wcloud version
```

## Before you open a pull request

CI runs lint, `go vet`, `go build`, `go test -race`, a cross-platform build and vet matrix
(Linux, macOS and Windows on amd64 and arm64), and `govulncheck`, on every push to `main` and
every pull request. Run the equivalents locally first; a red pipeline is slower than a local
loop.

- **Lint must be clean.** Fix the code rather than silencing the linter. If a `//nolint` is
  genuinely unavoidable, it needs an inline reason.
- **Tests come with the change.** New behaviour gets a test that fails without the change. Test
  conventions are in [`.claude/rules/testing.md`](./.claude/rules/testing.md).
- **Match the neighbouring code.** Architecture and layout rules are in
  [`.claude/rules/architecture.md`](./.claude/rules/architecture.md); several decisions there
  are settled and not open for relitigation.
- **Do not commit generated or release artifacts.** `dist/` is ignored outright; under `npm/`,
  only the copied platform binaries (`npm/weaviate-cloud-*/wcloud`) and pack tarballs
  (`npm/**/*.tgz`) are ignored — the platform package manifests and the shim script are tracked.

If your change touches a platform-specific path, say which platforms you tested on. The CI
matrix compiles for Windows and macOS but only runs the test suite on Linux.

## Commits and pull requests

Commit messages follow `<type>(<scope>): <short summary>`, lowercase, under 72 characters, no
trailing period. The scope is optional and free-form — a component name (`cluster`, `auth`), an
optional reference such as a GitHub issue number, or left out entirely when neither applies.
Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `ci`. Append `!` for a breaking change
(`feat!`, `fix!`, `chore!`).

```
feat(cluster): add delete cluster command
fix(#123): return 409 on duplicate cluster name
docs: add error-code column to the exit-code table
```

Branches follow the same idea: `fix/short-description`, or `fix/123-short-description` when
there's a reference worth keeping in the branch name. Don't invent one if there isn't.

One pull request per change, squash-merged into `main`. Never commit directly to `main` and
never skip pre-commit hooks with `--no-verify`. Fill in the pull request template: what
changed, the type of change, how to exercise it.

## Backend changes

When changing the public v1 surface of the Weaviate Cloud backend API, mention CLI impact in
the PR description. The CLI hand-rolls its HTTP client and models (no OpenAPI codegen for
v0.1), so additive backend changes are silent and breaking backend changes are caught only by
humans.
