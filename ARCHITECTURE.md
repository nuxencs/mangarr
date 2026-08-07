# ARCHITECTURE.md

Verified against code on 2026-03-25.

## Purpose

`mangarr` is a Go CLI for:

- one-shot manga chapter downloads
- continuous monitoring for newly released chapters
- packaging downloaded chapters into `.cbz` archives

## Product Domains

| Domain | Main entrypoints | Notes |
| --- | --- | --- |
| Download now | `cmd/download.go` | source selection, chapter selection, archive write |
| Continuous monitor | `cmd/monitor.go` | config load, ticker loop, dynamic reload, latest-chapter fetch |
| Source integration | `internal/source/` | highest churn; mixed API and HTML scraping |
| Chapter acquisition | `internal/acquire/`, `internal/download/`, `internal/files/` | naming, existing-file policy, page resolution, image fetch, CBZ creation |
| Ops + packaging | `.github/workflows/release.yml`, `.goreleaser.yaml`, `ci.Dockerfile` | release binaries and multi-arch Docker images |

## Package Layering

| Layer | Packages | Responsibility |
| --- | --- | --- |
| CLI surface | `main.go`, `cmd/` | parse flags, bootstrap commands, orchestrate flows |
| Application orchestration | `cmd/`, `internal/acquire/`, `internal/config/` | compose sources, acquire chapters, concurrency, config/runtime lifecycle |
| Core domain helpers | `internal/domain/`, `internal/parse/`, `internal/templater/`, `internal/sanitize/` | shared logic independent from any source |
| Integration layer | `internal/source/`, `internal/sharedhttp/`, `internal/download/` | fetch remote data and retry transient failures |
| Output + observability | `internal/files/`, `internal/logger/`, `internal/perf/`, `internal/buildinfo/` | archive write, logging, profiling, build metadata |

Rule: source-specific scraping logic stays in `internal/source/`. Generic retry, archive creation, and parsing stay shared.

## Runtime Flows

### `download`

1. Validate CLI input and destination path.
2. Construct source adapter from `-s`.
3. Fetch manga metadata and chapter list.
4. Resolve requested chapter set.
5. Pass each selected chapter to `internal/acquire/`.
6. Apply naming and existing-file policy.
7. Resolve and download pages concurrently.
8. Publish the `.cbz` archive.

### `monitor`

1. Load config and env overrides.
2. Initialize logger and optional `pprof`.
3. Start config reload watcher.
4. Tick on `checkInterval`.
5. For each monitored manga, resolve source adapter.
6. Pass the latest chapter to the shared acquisition module.
7. Report its downloaded or skipped outcome.

## Cross-Cutting Concerns

- Concurrency caps:
  - chapter jobs: `maxConcurrentChapterProcesses`
  - monitor source jobs: `maxConcurrentSourceProcesses`
  - image downloads: `maxConcurrentImageDownloads`
- Retry policy:
  - shared in `internal/sharedhttp/`
  - retries transport failures and `500/502/503/504`
  - fails fast on `404/429/401/403/405`

## Hotspots

- `internal/source/`: upstream site drift; brittle selectors; auth/rate-limit changes
- `cmd/monitor.go`: long-lived concurrency and shutdown behavior
- `internal/config/config.go`: config template, defaults, env overrides, live reload
- `internal/download/`: retry behavior and archive write seam

## Read Next

- [Design Guide](./docs/DESIGN.md)
- [Design Catalog](./docs/design-docs/index.md)
- [Product Specs](./docs/product-specs/index.md)
- [Plans Guide](./docs/PLANS.md)
- [Quality Score](./docs/QUALITY_SCORE.md)
