# User-Facing README Rewrite

Status: completed on 2026-03-27.

## Problem

The root README had drifted into a mixed user-plus-maintainer document. First-run guidance, config reference, runtime internals, and maintainer knowledge-base links all competed for attention.

## Scope

- rewrite the root README as a user-first landing page
- move deeper command/config/operator detail into a docs page
- keep maintainer and architecture detail in the existing internal docs
- update onboarding docs to match the new entrypoint flow

## Non-Goals

- CLI behavior changes
- config schema changes
- command help text cleanup

## Decisions

- keep the root README focused on install, quick start, monitor setup, and supported-source inputs
- add `docs/USAGE.md` as the deeper user/operator reference
- keep source identifier expectations in the README because they block first success
- keep maintainer links available, but secondary

## Progress Log

- reviewed the current README, architecture docs, and product onboarding docs
- verified command help and actual config lookup behavior against the code
- rewrote the README around user tasks instead of implementation detail
- added a dedicated usage/config reference doc for material removed from the root README
- updated the onboarding spec to reflect the new doc split

## Verification

- `go run . --help`
- `go run . download --help`
- `go run . monitor --help`
- manual review of README/doc links
- manual review of config-path wording against `internal/config/config.go`

## Follow-Up

- consider fixing CLI help text that says `--config` is a config file path; current runtime uses a config directory when the flag is set
