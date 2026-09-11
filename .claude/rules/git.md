# Git Rules

## Branch Naming

```
feat/short-description
fix/short-description
chore/short-description
refactor/short-description
docs/short-description
```

An optional reference — such as a GitHub issue number — may be worked into the description when
there's one worth keeping: `fix/123-return-409-on-duplicate-name`. Most maintenance, docs, and
chore branches have no such reference; don't invent one to fit the pattern.

Examples: `feat/add-delete-cluster-endpoint`, `fix/123-return-409-on-duplicate-name`

## Commit Messages

Format: `<type>: <short summary>`, or `<type>(<reference>): <short summary>` when there's a
reference worth including. The reference is optional and free-form — a GitHub issue number, an
identifier from whatever tracker you use, or nothing at all.

Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `ci`
Breaking changes: `feat!`, `fix!`, `chore!` — adds `BREAKING CHANGE` semantics.

Examples:
```
docs: add error-code column to the exit-code table
fix(#123): return 409 on duplicate cluster name
chore!: drop support for legacy status field
```

Keep summary under 72 chars, lowercase, no trailing period.

**Never add Claude Code or any AI tool as a co-author in commits.**

## Pull Requests

- Fill in `.github/pull_request_template.md` — description, type of change, related issue, related PRs, usage.
- One PR per change. No stacked PRs unless explicitly coordinated.
- PR title mirrors the commit message format.
- Never force-push to `main`. Feature branches are fine.
- Squash-merge into main to keep history clean.

## Never

- Never commit `.env` files (only `.env.example`).
- Never skip pre-commit hooks (`--no-verify`).
- Never commit directly to `main`.
