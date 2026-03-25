# Weeb Central HTTP Image Extraction

Status: completed on 2026-03-25.

## Problem

`weebcentral` chapter downloads were failing with `waiting for image page load: context deadline exceeded` because the adapter waited on the reader shell while chapter images arrived through a separate htmx fragment request.

## Scope

- replace browser-backed chapter image extraction with direct HTTP against the current chapter image fragment endpoint
- add regression coverage for the URL-construction and image extraction seam
- update docs that describe current adapter behavior

## Non-Goals

- browser package removal
- wider download/monitor orchestration cleanup

## Decisions

- keep `NewWeebCentral(...)` stable to avoid unrelated call-site churn
- fetch `/chapters/<id>/images?is_prev=False&current_page=1&reading_style=long_strip` directly
- parse returned HTML for ordered `img[src]` values and dedupe duplicates

## Progress Log

- confirmed live reader HTML loads image content through an htmx `/images` fragment
- confirmed the fragment is public and returns direct `<img src="...">` elements
- replaced rod/stealth image extraction with direct HTTP fragment fetching
- removed now-dead browser-manager plumbing from Weeb Central constructor/call sites
- added focused regression tests for query construction, query stripping, duplicate removal, and empty fragments
- updated source-behavior docs to match the new adapter path

## Verification

- `gofumpt -w internal/source/weebcentral.go internal/source/weebcentral_test.go`
- `go test ./internal/source/...`
- `go test ./...`
- `go test -race ./...`
- `go build ./...`
- `mkdir -p /tmp/mangarr-weebcentral-smoke`
- `go run . download -d /tmp/mangarr-weebcentral-smoke -s weebcentral -m https://weebcentral.com/series/01J76XYD7E91K8QP6CY0Y53900 -L`

## Follow-Up

- `internal/browser/` remains available in the repo for future JS-heavy sources, but no current adapter depends on it
