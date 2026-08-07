# Comix Protocol Restoration

## Problem

The Comix adapter targeted removed `/api/v2` endpoints and was disabled because current
API requests require a frontend-generated token. Research against frontend build
`35595e3de3c99889c1aa70` recovered request-token generation, encrypted-response decoding,
and scrambled-image reconstruction.

## Scope

- Add a source-local implementation of the current request and response codec.
- Move the Comix adapter to current `/api/v1` response shapes.
- Send required image request headers and reconstruct scrambled tile images.
- Add regression tests for protocol vectors, adapter behavior, image transport, and both
  tile-order algorithms.
- Restore Comix as a supported source with an explicit compatibility warning.

## Non-Goals

- Do not disguise Mangarr traffic or bypass account authentication.
- Do not enable signed-in Comix endpoints.
- Do not add automatic extraction of future frontend constants.

## Decisions

- Keep build-specific token and image behavior in `internal/source/`.
- Add `domain.ImageProcessor` as a narrow source-owned image transform seam. The downloader
  owns transport and output, but it does not import Comix behavior.
- Use fixed local fixtures for normal tests. Live Comix requests remain a manual smoke test.
- Treat `403` and `429` as unrecoverable through the existing shared HTTP policy.
- Keep optional numeric group selection. Without a group, the first duplicate chapter in
  server order wins because the domain chapter map has one value per chapter number.

## Completed Work

- [x] Reproduce the current request token from a fixed frontend vector.
- [x] Decode encrypted API response envelopes.
- [x] Port both embedded WebAssembly tile-order algorithms to pure Go.
- [x] Normalize direct and compact page payloads.
- [x] Send per-image referer headers and reconstruct scrambled pages as PNG.
- [x] Update source and user documentation.

## Verification

- `go test ./internal/domain ./internal/download ./internal/source -count=1`
- `go test ./...`
- `go test -race ./...`
- `go build ./...`
- `go vet ./...`
- `govulncheck ./...`: no vulnerabilities found
- `deadcode ./...`: no new Comix findings; it reported existing browser helpers,
  `MustParseChapterNumber`, and `CreatePDF`
- Live API smoke: title metadata, two-item chapter list, and chapter detail all returned and
  decoded through the Go implementation.
- Live CLI smoke: downloaded One Piece chapter 1190 from group `6594`, reconstructed its one
  scrambled page, wrote a 16-page CBZ, and passed `unzip -t` for every archive entry.
- Visual smoke: the reconstructed live page had coherent panels, text, and tile boundaries.

## Follow-Up Debt

- Frontend build drift can rotate token constants, response handling, hash prefixes, or tile
  algorithms without notice.
- Signed-in and mutation endpoints remain unsupported and untested.
- The domain chapter map still cannot represent duplicate chapter numbers from multiple
  groups at the same time.
