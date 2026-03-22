# Asura Scans Provider Repair

Status: completed on 2026-03-22.

## Problem

`asurascans` fetches failed because the adapter still targeted the retired `asuracomic.net/series/...` site shape and old browser selectors.

## Scope

- repair Asura Scans manga/chapter fetches against the current site
- require current `https://asurascans.com/comics/...` URLs
- add regression coverage for the new extraction seam
- update docs that describe adapter inputs/runtime behavior

## Non-Goals

- broad monitor runtime refactors
- repo-wide browser removal

## Decisions

- scrape current server-rendered `/comics/...` pages instead of relying on browser automation
- extract chapter page images from embedded chapter asset URLs in HTML

## Progress Log

- reproduced current failure with `download` and `monitor`
- confirmed live provider drift from `asuracomic.net` to `asurascans.com`
- identified current `/comics/.../chapter/...` page shape
- replaced the browser-backed Asura adapter with direct current-URL HTML scraping
- added focused regression tests for current series parsing and chapter image extraction
- updated the user monitor config to current Asura URLs before live polling

## Verification

- `gofumpt -w internal/source/asurascans.go internal/source/asurascans_test.go`
- `go fix ./...`
- `go test ./internal/source/...`
- `go run . download -d /tmp/mangarr-asura-smoke -s asurascans -m https://asurascans.com/comics/solo-max-level-newbie-7f873ca6 -L`
- `go run . download -d /tmp/mangarr-asura-smoke -s asurascans -m https://asurascans.com/comics/pick-me-up-infinite-gacha-7f873ca6 -L`
- `go run . monitor -c /Users/nuxen/.config/mangarr`
  - observed all six configured Asura entries process successfully after config update
- `go test ./...`
- `go test -race ./...`
- `go build ./...`

## Follow-Up

- `govulncheck ./...` still reports `GO-2026-4526` in `github.com/antchfx/xpath` via `colly`/`flamecomics`; unrelated to this Asura repair
