# RELIABILITY.md

Verified on 2026-03-07.

## Main Failure Modes

- upstream HTML/API changes break adapters
- remote rate limits or transient outages
- browser automation runtime issues
- invalid local config or download path
- partial chapter download failures

## Existing Controls

- shared retry policy in `internal/sharedhttp/`
- bounded concurrency via semaphores
- skip-on-existing archive behavior
- optional `pprof` endpoint for runtime inspection
- config reload without full process restart

## Verification Expectations

- narrow package tests while iterating
- full gate before handoff:
  - `go test ./...`
  - `go test -race ./...`
  - `go build ./...`
- adapter changes should include live-source smoke verification when possible

## Reliability Gaps

- no automatic live-source smoke suite
- no recurring docs/source drift audit
- monitor mode lacks a formal operator runbook
