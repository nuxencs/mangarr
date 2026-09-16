# Runtime Model

Verified against `cmd/download.go`, `cmd/monitor.go`, and `internal/config/config.go` on 2026-08-09.

## Commands

- `download`: one-shot flow; resolves a source, selects chapters, and delegates acquisition
- `monitor`: long-running flow; loads config, watches for changes, polls sources on an interval
- `version`: reports build metadata and latest GitHub release info

## State

Persistent state is file-based:

- downloaded archives on disk
- `config.yaml`
- optional log file output

There is no database, queue, or remote control plane.

Chapter acquisition (`internal/acquire/`) applies the
[existing-archive policy](../USAGE.md#download) before page resolution.
Images download to a temporary directory, then `internal/files/` assembles a
temporary CBZ beside the destination. After assembly, it syncs and closes the
temporary file, checks cancellation, and renames the file into place.

## Concurrency

- chapter jobs fan out in `download`
- source jobs fan out per tick in `monitor`
- image downloads fan out inside `internal/download`

Each layer has an explicit concurrency limit. The code favors bounded parallelism over unbounded goroutine fan-out.

## HTTP Retry Lifecycle

`internal/sharedhttp/` owns request retries for image downloads and adapters that
use shared direct HTTP, in both download and monitor mode. Transport failures
and retryable HTTP statuses use bounded retries with backoff and jitter. See
[retry.go](../../internal/sharedhttp/retry.go) for attempt and delay limits and
[http.go](../../internal/sharedhttp/http.go) for status handling.

A valid `Retry-After` value (seconds or HTTP date) can extend the fallback wait,
but cannot shorten it. Guidance above the maximum wait fails the request instead
of retrying before the server permits it. Cancellation interrupts retry waits.
The [retry regression tests](../../internal/sharedhttp/retry_test.go) cover these
constraints.

The policy adds no config or CLI settings and does not change concurrency or
monitor polling. It does not coordinate a provider-wide cooldown across requests
or processes. Persistent limits can still exhaust the retry budget. Colly-based
discovery and page requests have separate handling, although their image downloads
use shared retries. For user recovery steps, see
[bulk downloads and rate limits](../USAGE.md#bulk-downloads-and-rate-limits).

## Config Lifecycle

- defaults are embedded in Go structs/template text
- config path lookup checks the user config directory, `~/.mangarr`, then the binary directory
- env overrides use `MANGARR__` prefix
- configured downloads use `config.LoadExisting` and one snapshot; see [configured series downloads](../USAGE.md#download-a-configured-series) for requirements, selection, and override rules
- monitor mode publishes validated immutable config snapshots while running
- monitor mode watches the config parent directory, so atomic replacement and delete-then-recreate saves do not stop reloads
- invalid, incomplete, or temporarily missing config files keep the last valid snapshot active
- monitored manga, naming, download location, interval, and log level reload live
- pprof and log output destinations require a restart

## Operational Risk

The runtime model is simple, but upstream source drift is constant. Reliability depends more on adapter resilience and verification than on internal business rules.
