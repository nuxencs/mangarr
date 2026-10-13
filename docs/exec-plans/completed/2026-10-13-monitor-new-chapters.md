# Monitor downloads every new chapter

## Scope

Monitor selected only the highest chapter number on each check. If two chapters
released in one check interval, the lower one never downloaded. Fix this without
persistent state. Downtime gaps and a backlog command are out of scope.

## Decisions

- Keep an in-memory mark for each monitored series: the highest chapter number
  at its last successful check. Select the latest chapter plus every chapter
  above the mark. Without a mark, select latest only (unchanged first check).
- Move the mark only when no selected chapter failed, so failures retry.
- Set the mark to the current highest number, even when lower, so a removed
  upstream chapter cannot freeze selection.
- Reset the mark when source, manga, group, or language changes; drop it when
  the entry is removed. Overwrite changes keep it.
- Rejected: a SQLite chapter database with disk reconcile and ignore ranges.
  That belongs in the separate kanon project, which already has it. Rejected a
  disk walk to the newest archive, to keep monitor independent of naming and
  library layout.
- Share the bounded chapter pipeline between `download` and `monitor`
  (`cmd/chapters.go`).

## Verification

`TestMonitorDownloadsEveryNewChapter` uses a local Cubari fixture: first check
latest only, two releases in one interval, failure and retry. It fails when the
selection is reduced to latest only. Passed `go test ./...`,
`go test -race ./...`, `go build ./...`, and `git diff --check`.

Live on 2026-10-13 against TCB Scans One Piece: the built binary ran two checks
with a one-minute interval (check 1 downloaded 1195, check 2 skipped it at debug
level). A temporary harness ran the real check with the mark preset to 1192: 1193
and 1194 downloaded in parallel, 1195 was skipped, and the mark moved to 1195.
All archives passed `unzip -t`. No adapter changed.
