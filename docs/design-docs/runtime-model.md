# Runtime Model

Verified against `cmd/download.go`, `cmd/monitor.go`, and `internal/config/config.go` on 2026-03-07.

## Commands

- `download`: one-shot flow; resolves a source, selects chapters, downloads images, writes `.cbz`
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

Semaphores cap concurrency. The code favors bounded parallelism over unbounded goroutine fan-out.

## Config Lifecycle

- defaults are embedded in Go structs/template text
- config path lookup falls back through local and home-directory locations
- env overrides use `MANGARR__` prefix
- monitor mode can reload config while running

## Operational Risk

The runtime model is simple, but upstream source drift is constant. Reliability depends more on adapter resilience and verification than on internal business rules.
