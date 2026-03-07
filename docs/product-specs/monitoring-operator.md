# Monitoring Operator

Verified against `cmd/monitor.go` and `internal/config/config.go` on 2026-03-07.

## User Goal

Run `mangarr monitor` unattended and trust it to fetch new chapters without constant babysitting.

## Needs

- predictable config lookup and reload behavior
- visible logs and version/build context
- low-friction way to know when chapters were skipped, downloaded, or failed
- safe handling of flaky or drifting sources

## Acceptance Bar

- config behavior is documented and matches runtime
- monitor logs remain actionable for source failures
- failure handling does not corrupt already-downloaded archives
- browser-backed source requirements are documented
