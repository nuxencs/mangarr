# Runtime Model

Verified against `cmd/download.go`, `cmd/monitor.go`, and `internal/config/config.go` on 2026-09-16.

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

`internal/sharedhttp/` owns the request retry implementation. See the
[request retry policy and limits](../USAGE.md#bulk-downloads-and-rate-limits)
for command coverage, cancellation, and scraper limitations.

## Config Lifecycle

- defaults are embedded in Go structs/template text
- config path lookup checks the user config directory, `~/.mangarr`, then the binary directory
- env overrides use `MANGARR__` prefix
- ordinary downloads use `config.LoadDownload` and one read-only snapshot; missing implicit config is allowed, and only enabled logging settings are validated
- configured series use `config.LoadExisting` and require full config validation before flag overrides; see [configured series downloads](../USAGE.md#download-a-configured-series) for selection and override rules
- enabled file logging creates bounded per-run download files with OS locks for safe retention; see [download logs](../USAGE.md#download-logs)
- the Docker image links its config into binary-adjacent discovery so manual exec commands find the same settings as monitor
- monitor mode publishes validated immutable config snapshots while running
- monitor mode watches the config parent directory, so atomic replacement and delete-then-recreate saves do not stop reloads
- invalid, incomplete, or temporarily missing config files keep the last valid snapshot active
- monitored manga, naming, download location, interval, and log level reload live
- pprof and log output destinations require a restart

## Operational Risk

The runtime model is simple, but upstream source drift is constant. Reliability depends more on adapter resilience and verification than on internal business rules.
