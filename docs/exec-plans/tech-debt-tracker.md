# Tech Debt Tracker

Updated on 2026-03-07.

| Area | Debt | Impact | Next step |
| --- | --- | --- | --- |
| Docs governance | no doc link/index freshness checks in CI | docs can drift silently | add markdown/link/structure validation job |
| Doc gardening | no recurring stale-doc scan | dead docs accumulate | add scheduled doc-gardening automation |
| Source reliability | limited adapter regression fixtures | scraper drift reaches users faster | add adapter-focused fixture or smoke-test harness |
| Runtime observability | no documented health checklist for long-running monitor ops | failures harder to triage | add operator runbook and smoke checklist |
| Generated docs | `docs/generated/` is placeholder only | generated surfaces can rot later | define generation scripts once a generated artifact exists |
