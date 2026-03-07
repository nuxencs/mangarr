# Doc Knowledge Base Restructure

Status: completed on 2026-03-07.

## Problem

Repo docs were flat and useful, but not organized for progressive disclosure or durable agent execution context.

## Scope

- add root architecture map
- create indexed design and product spec areas
- add execution-plan folders and tech debt tracker
- add quality, reliability, security, and product-sense docs
- update root doc entry points

## Non-Goals

- CI doc linters
- generated doc automation
- code behavior changes

## Decisions

- keep `ARCHITECTURE.md` at repo root for fast discovery
- keep empty-ish domains such as frontend and db schema documented as `N/A today` to preserve stable paths
- use repo-specific reference files instead of generic framework placeholders

## Progress Log

- reviewed current README and maintainer docs
- mapped existing architecture/development/source content into new durable docs
- rewrote entry-point docs to use the new read order
- parked doc freshness automation under tech debt instead of forcing a half-built CI gate

## Verification

- manual review of file tree
- manual review of updated links and entry points
- no code/tests run; docs-only change

## Follow-Up

- add link/doc-structure linting
- add recurring doc-gardening automation
