# AGENTS.md

Agent guide for `mangarr`. Goal: high-signal changes, low entropy, docs/tests stay trustworthy.

## Start Here

Read in this order before changing behavior:

1. `README.md`
2. `ARCHITECTURE.md`
3. `docs/DESIGN.md`
4. `docs/PLANS.md`
5. `docs/QUALITY_SCORE.md`
6. `docs/design-docs/index.md`
7. `docs/product-specs/index.md`

Docs are the operator-facing system of record. If behavior changes, update docs in the same change.

## Repo Shape

- `cmd/`: CLI entrypoints and command orchestration.
- `internal/source/`: source adapters. Highest churn/risk area.
- `internal/download/`: image fetch/retry/concurrency.
- `internal/files/`: archive creation.
- `internal/config/`: config load/default/reload.
- `internal/browser/`: shared browser manager for JS-heavy sources.
- `internal/sharedhttp/`: HTTP client/retry policy.

## Harness Working Loop

1. Restate task + constraints.
2. Read docs + relevant code paths before editing.
3. Make a small plan. For non-trivial work, add or update a checked-in file under `docs/exec-plans/`.
4. Build the smallest diff that fixes root cause.
5. Verify with the narrowest useful test first, then full gate.
6. Update docs/tests while context is fresh.
7. Remove dead code, stale comments, and unused branches discovered in the touched area.

## Agent Legibility Rules

- Prefer explicit control flow over clever abstractions.
- Keep package boundaries clear. Do not leak source-specific behavior into generic packages unless it is truly shared.
- Add comments only for non-obvious invariants, scraping caveats, or retry/concurrency reasoning.
- Preserve or improve error context. Include source name, manga/chapter identifiers, and failing operation when useful.
- Keep files and functions readable. When touching a tangled area, leave it simpler than you found it.

## Source Adapter Rules

- Follow `domain.Source` contract exactly.
- Validate adapter input early and fail fast with actionable errors.
- Treat upstream HTML/API schemas as unstable. Avoid brittle selector chains when a more stable attribute exists.
- Reuse shared HTTP/browser utilities before adding adapter-local transport logic.
- If fixing a parser/selection bug, add or extend a regression test when practical.

## Verification

Minimum full gate before handoff:

```bash
go test ./...
go test -race ./...
go build ./...
```

Use narrower checks while iterating. For adapter changes, do manual smoke verification against the affected source when network access is available and note what was exercised.

## Dependency + Interface Changes

- New dependency: justify it. Check maintenance/release health first.
- Public CLI/config changes: update `README.md` and relevant docs in the same diff.
- Config behavior changes: keep `docs/design-docs/runtime-model.md`, [monitoring-operator.md](./docs/product-specs/monitoring-operator.md), and sample config usage aligned.

## Entropy Control

When you touch an area, look for easy cleanup:

- delete unreachable code
- collapse duplicate logic
- remove stale TODOs/comments
- tighten tests around the bug seam
- update docs that no longer match reality

Do not do opportunistic repo-wide rewrites.

## Handoff

Report:

- what changed
- what you verified
- any gaps, live-source limitations, or follow-up risks
