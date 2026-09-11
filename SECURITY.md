# Security Policy

## Reporting a vulnerability

**Do not open a public issue, pull request or discussion for a security vulnerability.**

Report it privately by opening a security advisory on this repository:
<https://github.com/weaviate/weaviate-cloud/security/advisories/new>. The advisory is visible
only to you and the maintainers until we publish it, so it is the right place for a
proof-of-concept, a token, a log excerpt or anything else you would not post in the open.

If you cannot reach that form, report it through the Weaviate console at
<https://console.weaviate.cloud> and say only that you are reporting a security issue in the
`wcloud` CLI; a maintainer will open a private channel with you. Withhold the details until
that channel exists.

We aim to acknowledge a report within three working days.

## What to include

- The version: paste the output of `wcloud version`.
- Your operating system and architecture.
- What an attacker gains, and what access they need to start.
- Steps to reproduce, or a proof-of-concept.

Redact your own credentials before sending. If a proof-of-concept needs a real token to
demonstrate the issue, say so rather than attaching one, and revoke anything you did attach
with `wcloud auth logout`.

## Scope

In scope: this repository, the `wcloud` binary built from it, and anything it does with
credentials on your machine. Credential handling is the area we care about most, in particular
the OAuth flow, the loopback callback listener, the on-disk credential file, and the allowlist
that decides which hosts a credential may be sent to.

Out of scope here: vulnerabilities in the Weaviate Cloud service itself, in a provisioned
Weaviate cluster, or in the console. Report those through <https://console.weaviate.cloud>.
Reports whose only finding is that the CLI requires read access to a private repository, or
that a user with root on their own machine can read their own credential file, are not
vulnerabilities.

## Supported versions

`wcloud` is in beta at v0.1. Fixes land on the latest release; there are no backports to
earlier versions.
