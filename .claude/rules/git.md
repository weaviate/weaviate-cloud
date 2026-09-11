# Git Rules

## Branch Naming

```
feat/<issue-id>-short-description
fix/<issue-id>-short-description
chore/<issue-id>-short-description
refactor/<issue-id>-short-description
docs/<issue-id>-short-description
```

Example: `feat/ISSUE-1234-add-delete-cluster-endpoint`

## Commit Messages

Format: `<type>(<issue-id>): <short summary>`

Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `ci`
Breaking changes: `feat!`, `fix!`, `chore!` — adds `BREAKING CHANGE` semantics.

Examples:
```
feat(ISSUE-1234): add delete cluster endpoint
fix(ISSUE-5678): return 409 on duplicate cluster name
chore!(ISSUE-999): drop support for legacy status field
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
