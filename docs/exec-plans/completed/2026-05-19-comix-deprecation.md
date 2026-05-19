# Comix Deprecation

## Problem

Comix moved chapter page access behind a browser-generated token. The public v1 manga and chapter-list endpoints still work, but `GET /api/v1/chapters/{id}` returns `403` with `Missing token.` outside the site runtime.

## Scope

- Keep `comix` as a recognized source key.
- Fail fast with a clear deprecation error.
- Update source docs and user-facing source tables.
- Add a regression test for the deprecation message.

## Non-Goals

- Reverse-engineer the token generator.
- Add a browser-backed Comix adapter.
- Remove the Comix source key in this change.

## Decisions

- Deprecate instead of repairing because token generation lives in obfuscated browser JavaScript and is likely to drift.
- Keep the source selectable so existing configs produce a targeted error instead of an unknown-source error.

## Verification

- `go test ./internal/source`
- `go test ./...`
- `go test -race ./...`
- `go build ./...`
