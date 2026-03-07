# QUALITY_SCORE.md

Baseline captured on 2026-03-07.

## Product Domains

| Domain | Score | Why | Gap to close |
| --- | --- | --- | --- |
| Download CLI | B | clear flow, decent docs, core tests around parsing/files/download | broader end-to-end smoke coverage |
| Monitor mode | B- | functional runtime model, config reload, logs | stronger runbook and operational checks |
| Source adapters | C+ | broad coverage, shared helpers | high drift risk, limited regression fixtures |
| Packaging/releases | B | GoReleaser + Docker CI in place | docs/CI ignore markdown-only drift; no doc checks |

## Architecture Layers

| Layer | Score | Why | Gap to close |
| --- | --- | --- | --- |
| CLI/orchestration | B | straightforward command layout | reduce some duplication between download and monitor selection flow |
| Core helpers | B | cohesive small packages | more tests around edge cases and naming semantics |
| Integration layer | C+ | shared retry/browser seams help | source drift remains dominant reliability risk |
| Output/observability | B- | logs and pprof exist | no formal health checklist or richer status summaries |

## Trend

This is the first recorded baseline. Update scores when material behavior or verification changes land.
