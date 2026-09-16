# Configured series downloads

## Goal and scope
Reuse `monitoredManga` entries from the existing download command by name, preserving chapter selectors and the explicit source/manga path. No force-download or rate-limit changes; no live-library downloads.

## Decisions
- Add `--series` for an exact, case-sensitive config key (quote names with spaces).
- Current config loading rules are documented in the [runtime model](../../design-docs/runtime-model.md#config-lifecycle).
- Explicit download flags override entry fields and global download/naming settings; environment overrides remain handled by the config loader. An omitted entry language retains the download default `en`.
- Existing full config validation still applies before download overrides; no monitor watcher or config rewrite.

## Plan
1. Resolve configured download defaults before existing source validation/discovery.
2. Add offline command regression tests for selection, precedence, invalid entries, and explicit usage.
3. Update help and usage/config docs and run the full Go gate.

## Progress
- Implemented exact-name `--series` selection with conditional CLI requirements and explicit flag overrides.
- Added offline command coverage for ranges, Cubari URL/group inputs, MangaDex IDs, default language, overrides, missing/unknown entries, and unchanged explicit usage.
- Updated help, README, usage reference, runtime/operator docs, and both config examples.

## Verification
- `go test ./cmd ./internal/config` passed.
- `go test ./...` passed.
- `go test -race ./...` passed.
- `go build ./...` passed.
- `go run . download --help` and `git diff --check` passed.
- No live providers or real library downloads exercised; tests use temporary config/output paths and fake discovery.

## Follow-up debt
None identified. Force re-download and bulk rate-limit work remain outside this change.
