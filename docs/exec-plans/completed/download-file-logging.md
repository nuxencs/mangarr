# Manual download file logs

## Goal
Retain manual download diagnostics whenever the existing file-logging setting is enabled. Preserve terminal output and keep monitor files separate.

## Scope and decisions
- Read config once through `internal/config`, including default discovery and environment overrides. Require an existing config for explicit selection or `--series`; allow ordinary downloads without config. Validate only enabled logging settings for ordinary downloads. Preserve full config validation for `--series`.
- Keep the image's existing config layout. Expose its config through the existing binary-adjacent discovery location with a symlink, rather than adding global lookup rules or another enable setting.
- Store collision-resistant JSONL run files in `<logPath>.downloads/`. Keep the monitor's file and rotation unchanged.
- Cap each run at `logMaxSize` MiB. Retain `logMaxBackups` completed runs plus active runs. Use OS file locks to protect active runs and serialize creation/cleanup. Process exit releases locks, so interrupted runs become eligible for cleanup.
- Report a size limit or write failure on stderr and at command completion. Continue acquisition and preserve its errors and output files. Reject startup logging failures before discovery.
- Retain emitted diagnostics without additional record filtering. Keep existing source safeguards, console formatting, and verbosity unchanged.

## Plan
1. Add offline command coverage for the missing retained discovery error.
2. Add read-only download config loading and command-local logging lifecycle.
3. Test concurrent processes, bounded retention, output separation, and failure details.
4. Update operator docs and verify affected packages, then the required Go gates.
5. Commit the implementation and stop for Firstmate's validation instruction.

## Verification
- Reproduced missing file diagnostics with an offline discovery failure before implementation. The command regression verifies one stderr error, a retained failure cause, and empty stdout.
- Configured-series fixtures retain successful acquisitions and later skips. Partial-failure fixtures retain both the successful chapter and failed-chapter summary.
- Automatic config and binary-adjacent symlink tests require no new logging option. Monitor settings remain independently validated.
- Subprocess tests verify unique files, active-run protection, concurrent cleanup, and lock release after abrupt exit. Other coverage checks size limits, write failures, retained failure details, permissions, and symlink preservation.
- `go test ./...`, `go test -race ./...`, and `go build ./...` passed. Affected package tests and logger race tests passed again after final file-name/path guards.
- `go vet ./cmd ./internal/config ./internal/logger`, `go mod tidy -diff`, `yamllint config.yaml`, and `git diff --check` passed.
- Logger tests cross-compiled for Linux, Windows, and FreeBSD. Their foreign-platform binaries were not executed.
- A release-style native binary found a config through the image-equivalent symlink. An invalid fixture source returned status 1, printed the destination and error once on stderr, left stdout empty, retained JSON diagnostics, and did not create the monitor file.

No provider traffic, real downloads, server changes, or validation pipeline runs occurred. The container image was not built or run. Firstmate will start the required no-mistakes validation after the implementation commit; merge still requires separate approval.
