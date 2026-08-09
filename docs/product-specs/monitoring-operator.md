# Monitoring Operator

Verified against `cmd/monitor.go` and `internal/config/config.go` on 2026-08-09.

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
- source-specific runtime requirements stay documented when they appear
- monitor checks configured manga once at startup before waiting for the interval
- invalid reloads keep the last valid config snapshot active
- atomic replacement and delete-then-recreate saves do not stop future reloads
- config lookup follows the documented operating-system and binary locations
