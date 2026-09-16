# RELIABILITY.md

Verified on 2026-08-07.

## Main Failure Modes

- upstream HTML/API changes break adapters
- remote rate limits or transient outages
- source-specific transport or anti-bot changes
- invalid local config or download path
- partial chapter download failures

## Existing Controls

- shared retry policy in `internal/sharedhttp/`, including bounded HTTP 429 retries
  and cancellable `Retry-After` waits (see [usage](./USAGE.md#bulk-downloads-and-rate-limits))
- bounded chapter, source, and image concurrency
- one shared acquisition policy for download and monitor
- [existing-archive policy](./USAGE.md#download)
- [archive publication sequence](./design-docs/runtime-model.md#state)
- optional `pprof` endpoint for runtime inspection
- immediate first monitor poll and reload-aware scheduling
- validated, immutable config snapshots without full process restart
- fixture-backed regression flows for every source adapter
- CI tests, race tests, builds, vets, vulnerability scans, and repository-file checks

## Verification Expectations

- narrow package tests while iterating
- full gate before handoff:
  - `go test ./...`
  - `go test -race ./...`
  - `go build ./...`
- adapter changes should include live-source smoke verification when possible

## Reliability Gaps

- retries do not coordinate a host-wide cooldown; sustained limits can exhaust the budget
- Colly discovery/page requests do not use the shared retry policy
- no automatic live-source smoke suite
- no recurring docs/source drift audit
- monitor mode lacks a formal operator runbook
