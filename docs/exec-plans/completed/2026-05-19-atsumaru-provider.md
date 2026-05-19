# Atsumaru Provider

## Problem

Add `atsu.moe` as a source provider so users can download chapters from a manga URL such as `https://atsu.moe/manga/Q5Mqy`.

## Scope

- Added source key `atsumaru`.
- Accepted full Atsu manga URLs through `-m` and required scan IDs through `-g`.
- Used Atsu JSON APIs for manga info and chapter pages.
- Preserved API chapter titles.
- Updated docs and added adapter regression tests.

## Decisions

- Raw manga IDs are not accepted as public input.
- `-g` maps to Atsu `scanId` and filters duplicate chapter numbers by scanlation upload.
- `forceStrip` maps to `domain.Manga.IsManhwa`.
- Page image paths are resolved against `https://atsu.moe`.
- No new dependency was needed.

## Verification

- `go test ./internal/source`
- `go test ./...`
- `go test -race ./...`
- `go build ./...`
- `mkdir -p /tmp/mangarr-atsumaru-smoke`
- `go run . download -d /tmp/mangarr-atsumaru-smoke -s atsumaru -m https://atsu.moe/manga/Q5Mqy -g cmgzlsevifjhtm191rqugvee3 -L`

Live smoke produced:

- `/tmp/mangarr-atsumaru-smoke/Kagurabachi/Kagurabachi Ch. 121 - Chapter 121.cbz`
