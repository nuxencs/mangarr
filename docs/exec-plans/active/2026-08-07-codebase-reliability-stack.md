# Intent Lock - Codebase Reliability Stack

**Goal:** Improve Mangarr's reliability, maintainability, testability, and delivery
confidence without changing its product scope. Ship the work as a dependency-ordered
stack of small draft pull requests.

**Core decision:** Make the existing CLI dependable enough that archive failures,
monitor reloads, source drift, and release automation fail visibly and safely.

**Success criteria:**

- Every pull request is independently reviewable and leaves its branch buildable.
- Every pull request passes `go test ./...`, `go test -race ./...`, and
  `go build ./...`.
- A failed archive write never leaves a final file that a later run treats as
  complete.
- Monitor config reload is race-free, validates new snapshots before publication,
  and applies documented reloadable settings.
- Monitor performs its first poll immediately.
- `mangarr version` reports local build data successfully without network access.
- Command and config behavior has regression coverage through their public seams.
- Source discovery and page resolution use a small, truthful interface with no
  required no-op methods.
- Download and monitor use one chapter-acquisition implementation for shared
  behavior.
- CI enforces the repository gate and validates important documentation and
  infrastructure files.
- Unreachable browser, PDF, and helper code and their dependencies are removed.
- User and maintainer documentation matches the final behavior and Go version.

**Non-goals:**

- New manga sources or new download features.
- A web interface, database, queue, plugin system, or remote control plane.
- A repo-wide style rewrite or unrelated package reorganization.
- Changes to source-specific behavior unless required for a confirmed bug,
  cancellation, fixture testability, or the new source interface.
- New abstraction layers without at least two real callers or adapters.

**Technical givens:** Go 1.26; Cobra CLI; existing file-based configuration and
archive storage; GitHub Actions, GoReleaser, and Docker release targets; `develop`
as the stack base; GitHub CLI for all GitHub operations; each draft pull request
targets the preceding stack branch; documentation and tests ship with the behavior
they describe.

## Buckets

1. **CI baseline** - Add a least-privilege pull-request gate for tests, race tests,
   build, vet, vulnerability scanning, module consistency, generated-code checks,
   and relevant documentation and infrastructure validation.
2. **Atomic archive publication** - Publish validated CBZ files by same-directory
   temporary file and rename, report close and flush failures, validate destination
   directories, and cover failure paths.
3. **Monitor configuration and lifecycle** - Replace shared mutable config with
   validated snapshots, make reload behavior explicit, update the timer safely,
   poll immediately, and test reload and cancellation.
4. **Adapter reliability baseline** - Add fixtures for TCB Scans, Flame Comics,
   and MangaDex; fix MangaDex filtered pagination; improve context cancellation and
   exact-host validation where covered by fixtures.
5. **Command seam** - Construct Cobra commands from dependencies, return errors
   instead of exiting in command implementations, test primary command outcomes,
   and make the version update check advisory.
6. **Source interface and registry** - Replace mutation and no-op lifecycle methods
   with discovery and page-return methods, and use one source registry from both
   command paths.
7. **Chapter acquisition module** - Move naming, existing-file policy, page
   resolution, download, archive publication, and explicit outcomes behind one deep
   module used by download and monitor.
8. **Entropy and dependency cleanup** - Remove unreachable browser, PDF, and helper
   code; remove shallow one-adapter interfaces where they add no leverage; tidy the
   module graph; add a reproducible protobuf generation check.
9. **Container and documentation alignment** - Make the Compose sample safe when
   variables are absent, reduce and pin the runtime image where practical, align
   config lookup and Go-version documentation, and update architecture, reliability,
   quality, and operator docs.

## Build Log

## Bucket 1 - CI baseline - ALIGNED

Built: Added least-privilege CI, repository-file validation, and reproducible
protobuf generation.

Serves goal/decision because: Every later stack branch now has one explicit gate
for code, generated output, dependencies, documentation, and release inputs.

Notes/risks: CI installs pinned Go and YAML tools. GitHub actions follow the
repository's existing major-version pinning convention.

## Bucket 2 - Atomic archive publication - ALIGNED

Built: Archive output now uses a same-directory temporary file, verifies ZIP and
file completion, and renames only after success. Empty archives and non-directory
destinations fail validation.

Serves goal/decision because: Failed writes can no longer leave a final CBZ that
later monitor or download runs mistake for a completed chapter.

Notes/risks: Atomic replacement follows the host filesystem's rename semantics.
