# Testing Rules

## Structure

Tests live alongside the code they test (`_test.go` suffix). Use the `_test` package (black-box) for anything user-visible — keeps the public surface honest. Same-package tests are fine for genuinely internal helpers.

Use table-driven tests for multi-case behaviour. Name the cases descriptively; one failure should localise the broken case without rerunning under a debugger.

## Default to TDD

For new behaviour: write the failing test, watch it fail, make it pass, then refactor. Shipping production code without a test is the exception, not the norm — call it out explicitly when you skip.

Run the targeted package while iterating (`go test ./internal/foo/...`), then the full suite (`go test ./...`) before pushing.

## CLI command tests

Drive cobra commands through `cobra.Command.Execute()` with `SetArgs(...)`, using a `*factory.Factory` whose `Now` and `NewRequestID` are mocked for determinism. Capture stdout/stderr via `iostreams.Test()` and assert against the JSON envelope structure — not against substring matches that will break when the envelope evolves.

## Mocking

When a port needs a mock (HTTP client, auth provider, etc.), prefer:

- Small hand-written fakes for one-off tests with a single call site.
- `mockery`-generated mocks (pinned via `mise.toml`) when the same port has many test callers. Generated mocks live next to the interface (`mocks/` subpackage). Never write mocks by hand for an interface that already has a mockery target.

Always assert both the success/error envelope shape AND the metadata (`api_version`, `request_id` non-empty). The envelope is the public contract.

## What not to test

- `cmd/wcloud/main.go` wiring (covered by running the binary).
- Trivial getters/setters with no logic.
- Standard-library behaviour (`encoding/json` semantics, `time.Now`).
- Anything an integration / end-to-end test already covers — no duplicate unit tests for the same code path.
