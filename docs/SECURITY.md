# SECURITY.md

Verified on 2026-08-07.

## Trust Boundaries

- untrusted remote HTML/API responses from manga sources
- local filesystem writes for archives and optional logs
- outbound HTTP access to third-party manga sources
- GitHub release and container publishing pipeline

## Current Posture

- no multi-user server surface
- no database or credential store in repo code
- primary security risks come from third-party content and supply chain dependencies

## Safe Defaults

- validate source inputs before network work
- keep archive output under explicit user-selected directories
- prefer shared HTTP code paths over ad hoc adapter-local clients
- document new dependencies before adoption
- scan reachable dependencies with `govulncheck` in CI
- run the container as an unprivileged user with a pinned Alpine release line

## Gaps

- no dedicated egress allowlist or source isolation controls
- GitHub actions use version tags rather than immutable commit digests
