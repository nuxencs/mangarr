# AVIF chapter downloads

## Problem

The deployed monitor failed to download One Punch Man chapter 239 from Atsumaru on 2026-09-14.
The live CLI reproduces `unsupported content type: image/avif`.

## Scope

- Support AVIF pages through download, image size checks, and CBZ creation.
- Preserve original page bytes and existing Manhwa filtering.
- Verify the fix locally and deploy it to the running Mangarr service.
- Leave recovered upstream Comix and MANGA Plus failures unchanged.

## Decisions

- Verify the complete download path against the live source.
- Use an established AVIF decoder instead of a custom container parser.
- Use `github.com/gen2brain/avif` v0.6.0 for archive size checks. The project is
  active (latest repository update: 2026-08-14) and provides a CGo-free fallback.
  This avoids a custom AVIF parser and preserves Manhwa filtering. It adds
  `purego` and `wazero` as indirect dependencies and increases binary size.

## Progress

- Confirmed 81 repeated AVIF failures in the previous 72 hours.
- Reproduced chapter 239 failure with the deployed CLI.
- Found both an absent MIME mapping and an archive size check that requires a decoder.
- Added the MIME mapping and registered the decoder.
- Do not retain format-specific README notes, fixtures, or image-data tests,
  as requested by the user.
- Use `nodynamic` for release and container builds. The optional native library
  loader fails to compile for FreeBSD with CGo disabled under Go 1.27. On Linux,
  it also introduces a glibc loader requirement that the Alpine container cannot satisfy.
- Deployed a temporary patched image through the service's Compose configuration.
- The first monitor cycle downloaded chapter 239 successfully at 16:03 CEST.

## Verification

- `go test ./...`, `go test -race ./...`, `go build ./...`, and `go vet ./...` pass.
- `govulncheck ./...`: no vulnerabilities found.
- `go mod tidy -diff`, `goreleaser check`, YAML lint, and `git diff --check` pass.
- All seven release targets compile with `CGO_ENABLED=0` and `-tags nodynamic`.
- The patched CLI downloaded chapter 239 in the deployed container environment.
  The resulting CBZ passed `unzip -t`.
- The replacement monitor downloaded the chapter to the library. Comix and
  MANGA Plus checks also succeeded. The container has no restarts.
- No new image-data tests retained, per user instruction.

## Deployment result

After the release build passed, the service returned to
`ghcr.io/nuxencs/mangarr:develop` and the temporary pull policy was removed.
Chapter 238 was also downloaded. Both chapter archives passed integrity checks,
and all 19 monitored manga checks completed without warnings or errors.
