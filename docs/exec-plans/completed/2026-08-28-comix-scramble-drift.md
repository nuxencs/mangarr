# Comix Scramble Drift Repair

## Problem

Comix discovery and chapter resolution still work, but live image processing
fails on new `X-Scramble-Hash` values. The adapter rejects hashes that are not
in its two-entry prefix table.

## Scope

- Reproduce the live latest-chapter failure.
- Compare current frontend hash handling with the Go adapter.
- Add a regression test at the image-processing seam.
- Restore a complete live CBZ download and inspect the reconstructed page.
- Update the adapter design and reverse-engineering record.

## Non-Goals

- Change the Comix request token or response codec.
- Add a browser dependency to Mangarr.
- Modernize unrelated Go code.

## Decisions

- Mirror the frontend's zero-prefix fallback for unmapped hashes.
- Keep the two explicit legacy mappings because the active bundle still uses
  them.
- Use a synthetic tile image for regression coverage. Do not store live page
  content in the repository.

## Progress

- Reproduced failures for live hashes `a8284` and `e05d1`.
- Inspected the active first-party security bundle and its runtime lookup.
- Confirmed that zero is the correct prefix through tile-seam scoring.
- Added a failing regression case, implemented the fallback, and made it pass.
- Completed a live latest-chapter archive and visual page inspection.

## Verification

- `go test -v ./internal/source`
- `go test ./...`
- `go test -race ./...`
- `go build ./...`
- `git diff --check`
- Live latest One Piece chapter download: 13-page, 15 MB CBZ
- Full-resolution visual inspection of reconstructed page 10
