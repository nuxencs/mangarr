# Chapter Number Refactor

Status: completed on 2026-03-07.

## Problem

Chapter numbers were represented as `float32` across the domain model, selection logic, adapter parsing, and templating. That lost decimal precision, required epsilon-based comparisons, and made chapter identity depend on float behavior.

## Scope

- add an exact decimal `ChapterNumber` domain type
- replace float-based chapter-number storage and comparisons
- update adapters, parsing, templating, and command flows to use the new type
- add regression tests for parsing, comparison, and formatting
- record the execution plan and completion details in repo docs

## Non-Goals

- support non-numeric chapter suffixes such as `12a`
- preserve the exact upstream textual representation of chapter numbers
- redesign chapter storage away from maps

## Decisions

- represent chapter numbers as `whole`, `fraction`, and `scale` integers
- canonicalize output by trimming insignificant trailing fractional zeroes
- keep the CLI/config/template surface unchanged
- keep chapter storage as `map[domain.ChapterNumber]domain.Chapter`

## Progress Log

- added `domain.ChapterNumber` with exact parsing, comparison, formatting, and JSON decoding
- moved the domain model, chapter selection, summary formatting, templating, and download/monitor flows off float semantics
- updated all source adapters to parse or decode chapter numbers at the boundary
- removed the float padding helper and replaced it with type-owned formatting
- added regression tests for the new type, selection behavior, and template formatting

## Verification

- `go test ./...`
- `go test -race ./...`
- `go build ./...`

## Follow-Up

- consider a later slice-backed chapter collection if map-key lookups become limiting
- add live-source smoke checks for at least one decimal-chapter source when convenient
