# Design Docs Index

Catalog verified on 2026-03-07.

## Catalog

| Doc | Status | Verification | Use when |
| --- | --- | --- | --- |
| [core-beliefs.md](./core-beliefs.md) | active | reviewed against current repo goals | deciding tradeoffs, guardrails, cleanup bar |
| [runtime-model.md](./runtime-model.md) | active | checked against `cmd/` and `internal/config/` | changing command orchestration or runtime behavior |
| [source-adapters.md](./source-adapters.md) | active | checked against `internal/source/` | touching source inputs, selectors, browser usage |

## Verification Rules

- mark docs with last verification date
- update the relevant doc in the same change when behavior shifts
- prefer a small number of durable docs over many shallow notes

## Next Reads

- [Architecture](../../ARCHITECTURE.md)
- [Product Specs Index](../product-specs/index.md)
- [Quality Score](../QUALITY_SCORE.md)
