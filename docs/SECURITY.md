# SECURITY.md

Verified on 2026-03-07.

## Trust Boundaries

- untrusted remote HTML/API responses from manga sources
- local filesystem writes for archives and optional logs
- optional browser automation against third-party sites
- GitHub release and container publishing pipeline

## Current Posture

- no multi-user server surface
- no database or credential store in repo code
- primary security risks come from third-party content, browser automation, and supply chain dependencies

## Safe Defaults

- validate source inputs before network work
- keep archive output under explicit user-selected directories
- prefer shared HTTP/browser code paths over ad hoc adapter-local clients
- document new dependencies before adoption

## Gaps

- no documented vuln-scanning routine beyond normal dependency hygiene
- no sandboxing around browser-backed adapters
- no CI checks focused specifically on dependency or docs-governance risk
