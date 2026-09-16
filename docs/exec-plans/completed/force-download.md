# Force chapter re-download

## Scope

Add an explicit download-only `-f/--force` flag that re-acquires only selected
chapters. Ordinary downloads and monitoring continue to skip existing archives.
Bulk throttling and configured-series selection are out of scope.

## Decisions and plan

- Pass force through CLI options to the shared acquisition request; its archive
  stat is the only existing-file short-circuit on this path.
- Reuse temporary image downloads and atomic CBZ publication. Add cancellation
  checks during assembly and before publication so the previous archive survives
  a failed or cancelled replacement.
- Cover default skip, selected-only replacement, fetch/assembly failures and
  cancellation with temporary directories and local HTTP fixtures.
- Update help and usage; run narrow tests followed by all tests, race and build.

## Progress

- Read required docs and traced acquisition, image download and archive publication.
- Implemented download-only force propagation and cancellation-aware assembly.
- Added CLI fixture coverage for ordinary skip, explicit/first/latest/all forced
  selection, archive contents and untouched unselected files.
- Added acquisition failure coverage (page resolution, fetch, malformed image,
  cancellation) and atomic writer coverage proving originals survive failure
  or cancellation and remain readable during assembly.
- Updated CLI help, README, usage and runtime model. Ran the required project
  memory helper; retained its maintenance section and CLAUDE.md pointer.

## Verification

Passed `go test ./cmd ./internal/acquire ./internal/files`, `go test ./...`,
`go test -race ./...`, `go build ./...`, and `git diff --check`.
Confirmed `go run . download --help` exposes `-f/--force`.
All downloads exercised local HTTP fixtures and temporary directories; no live
source or real library was used. No source adapter behavior changed.

## Follow-up debt

None identified for this feature. Independent no-mistakes validation follows
implementation handoff.
