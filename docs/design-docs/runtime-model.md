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

## Concurrency

- chapter jobs fan out in `download`
- source jobs fan out per tick in `monitor`
- image downloads fan out inside `internal/download`

Each layer has an explicit concurrency limit. The code favors bounded parallelism over unbounded goroutine fan-out.

## Config Lifecycle

- defaults are embedded in Go structs/template text
- config path lookup checks the user config directory, `~/.mangarr`, then the binary directory
- env overrides use `MANGARR__` prefix
- monitor mode publishes validated immutable config snapshots while running
- monitor mode watches the config parent directory, so atomic replacement and delete-then-recreate saves do not stop reloads
- invalid, incomplete, or temporarily missing config files keep the last valid snapshot active
- monitored manga, naming, download location, interval, and log level reload live
- pprof and log output destinations require a restart

## Operational Risk

The runtime model is simple, but upstream source drift is constant. Reliability depends more on adapter resilience and verification than on internal business rules.
