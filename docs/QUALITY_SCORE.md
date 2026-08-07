# QUALITY_SCORE.md

Assessment updated on 2026-08-07 after the reliability stack.

## Product Domains

| Domain | Score | Why | Gap to close |
| --- | --- | --- | --- |
| Download CLI | A- | one acquisition policy, explicit outcomes, end-to-end archive coverage | live smoke coverage remains manual |
| Monitor mode | B+ | immediate polling, validated reload snapshots, shared acquisition | formal operator runbook and health checks |
| Source adapters | B | small return-value contract, one registry, fixture flow for every source | upstream drift still requires live verification |
| Packaging/releases | A- | full CI gate, generated-code check, pinned lean runtime image | actions are not pinned to commit digests |

## Architecture Layers

| Layer | Score | Why | Gap to close |
| --- | --- | --- | --- |
| CLI/orchestration | A- | fresh command trees, one source registry, shared acquisition boundary | command coverage can expand around multi-chapter summaries |
| Core helpers | B+ | cohesive packages, bounded concurrency, dead production code removed | naming syntax remains custom and lightly documented |
| Integration layer | B | shared retry, cancellation, exact-host validation, offline fixtures | source drift remains the dominant reliability risk |
| Output/observability | A- | atomic CBZ publication, explicit summaries, logs, pprof | no formal health checklist |

## Trend

The stack improved every recorded domain. Future score changes need code, test, or
operational evidence, not documentation-only assertions.
