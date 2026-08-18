# Asura Scans Premium Lock Repair

Status: completed on 2026-08-18.

## Problem

Asura Scans replaced its chapter `is_locked` payload field with `is_premium` and
`early_access_until`. Mangarr no longer filters premium chapters from discovery,
so monitor mode repeatedly tries to resolve image URLs that are not public yet.

Manga Plus request errors also include full request URLs. Registration and
chapter requests can therefore expose secret query values in normal logs.

## Scope

- support current and legacy Asura locked-chapter fields
- add regression coverage for current premium chapter payloads
- remove Manga Plus query values from request error context
- document the source behavior and log-safety guarantee

## Non-Goals

- retry successful Asura responses that temporarily contain no chapter links
- change monitor scheduling or general retry policy
- change server configuration or deploy the repaired image

## Diagnosis

- `Pick Me Up, Infinite Gacha` failed four monitor checks from 18:11 through
  18:56 on 2026-08-18, then resumed normal chapter 214 checks at 19:11 without a
  process restart.
- `The Stellar Swordmaster` returned the same empty-chapter error during the
  first half of that interval, which confines the failure to a shared Asura
  upstream response window rather than one configured title.
- The current Asura payload marks chapter 134 with `is_premium=true` and a future
  `early_access_until`; the chapter page contains no public image assets.
- The adapter still matches only `is_locked=true`.
- Manga Plus error logs contain the complete registration URL, including query
  values that must not be logged.

## Decisions

- treat either `is_locked=true` or `is_premium=true` as unavailable during
  discovery
- retain old-field compatibility because Asura payloads are unstable
- omit the complete query string from Manga Plus request errors
- classify the bounded Pick Me Up incident as transient upstream behavior; the
  existing 15-minute monitor loop already recovered without intervention

## Progress Log

- reproduced the premium chapter selection failure from server logs and public
  Asura payload metadata
- confirmed a previously failing premium chapter now exposes images after its
  lock expired
- isolated the Pick Me Up errors to four checks in one shared Asura outage window
- added current and legacy Asura lock-field coverage
- removed Manga Plus query strings from request error context
- updated source-adapter and security documentation

## Verification

- regression tests failed before the fix for both production symptoms
- `go test ./internal/source -run 'TestAsurascansDiscoverSkipsPremiumChapters|TestMangaPlusRequestErrorOmitsQueryValues' -count=1`
- `go test ./internal/source/... -count=1`
- `go build ./...`
- `go test ./...` reached and passed all changed packages; four unrelated
  `internal/config` tests fail because Windows temporary paths are inserted into
  YAML double-quoted strings without escaping
- `go test -race ./...` could not start because CGO is disabled
- a focused race retry with CGO enabled could not start because `gcc` is not
  installed
- `gofumpt` was unavailable; changed Go files were formatted with `gofmt`

## Follow-Up

- deploy a new container image after the code change is reviewed and published
- repair Windows path quoting in the existing config tests
- run the race gate in CI or another environment with CGO and a C compiler
