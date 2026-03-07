# PLANS.md

Verified on 2026-03-07.

Plans are first-class repo artifacts.

## Plan Levels

- small change: lightweight thread-local plan is enough
- non-trivial change: add a checked-in execution plan under `docs/exec-plans/active/`
- finished work: move or recreate the plan under `docs/exec-plans/completed/` with decisions and verification

## Plan Expectations

Each execution plan should capture:

- problem statement
- scope and non-goals
- decisions made
- progress log
- verification performed
- follow-up debt if any

## Current Plan Areas

- [Active Plans](./exec-plans/active/README.md)
- [Completed Plans](./exec-plans/completed/README.md)
- [Tech Debt Tracker](./exec-plans/tech-debt-tracker.md)

## Mechanical Follow-Through

This repo does not yet enforce doc freshness mechanically. Track missing linters, link checks, and doc-gardening automation in the tech debt tracker until implemented.
