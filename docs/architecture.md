# Architecture

## Goal

`mangarr` downloads manga chapters into `.cbz` archives and can continuously monitor sources for newly released chapters.

## Entry Points

- `main.go`: boots CLI via Cobra.
- `cmd/download.go`: one-shot chapter download flow.
- `cmd/monitor.go`: long-running monitor loop using `config.yaml`.
- `cmd/version.go`: local build metadata + latest GitHub release check.

## Runtime Flow

### Download command

1. Validate CLI flags and download destination.
2. Select source adapter (`internal/source`).
3. Fetch manga metadata and chapter list.
4. Resolve chapter selection (first/latest/list/range/all).
5. Skip existing archive files.
6. Fetch image URLs for each selected chapter.
7. Download images concurrently (`internal/download`) with targeted retry policy.
8. Build chapter archive via `internal/files.CreateCbzArchive`.

### Monitor command

1. Load config (`internal/config`), enforce defaults, apply env overrides.
2. Start optional `pprof` endpoint (`internal/perf`).
3. Start ticker loop (`checkInterval` minutes).
4. For each configured manga, select source adapter and fetch latest chapter.
5. Skip if `.cbz` already exists.
6. Download + archive latest chapter.
7. Watch `config.yaml` for live reload.

## Concurrency Model

- `cmd/download.go`: chapter-level concurrency capped by `maxConcurrentChapterProcesses` (10), with 2s pacing delay between non-skipped chapter jobs.
- `cmd/monitor.go`: source-level concurrency capped by `maxConcurrentSourceProcesses` (10), driven by a ticker interval.
- `internal/download/download.go`: image-level concurrency capped by `maxConcurrentImageDownloads` (10), each request using shared retry policy.

## Package Layout

- `cmd/`: CLI commands and orchestration.
- `internal/source/`: source adapters (scrape/API per site).
- `internal/download/`: image fetch + decryption + retry behavior.
- `internal/sharedhttp/`: shared HTTP transport, status-code handling, and retry policy configuration.
- `internal/files/`: archive creation and image filtering logic.
- `internal/config/`: config load/default/env/dynamic-reload.
- `internal/browser/`: Rod browser manager for JS-heavy sources.
- `internal/parse/`: chapter selection parsing utilities.
- `internal/templater/`: naming template execution.
- `internal/logger/`: zerolog + optional file log sink.
- `internal/perf/`: optional pprof server.
- `internal/domain/`: shared structs/interfaces.

## HTTP Retry Policy

- Attempts: 3 total (2 retries), delay `1s`, max jitter `250ms`.
- Retryable failures: transport errors and `500/502/503/504`.
- Fail-fast (no retry): `404/429/401/403/405` and other unexpected status codes.

## Data Contracts

- Source adapters implement `domain.Source` with `ValidateInput()`, `GetManga(ctx)`, `GetChapters(ctx, manga)`, and `GetImageURLs(ctx, chapter)`.
- `domain.Config` drives monitor behavior: download path, naming template, check interval, pprof/log settings, and `monitoredManga` source definitions.
