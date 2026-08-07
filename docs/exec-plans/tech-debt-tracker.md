# Tech Debt Tracker

Updated on 2026-08-07.

| Area | Debt | Impact | Next step |
| --- | --- | --- | --- |
| Doc gardening | no recurring stale-doc scan | dead docs accumulate | add scheduled doc-gardening automation |
| Source reliability | no automated live-source smoke suite | upstream drift can pass stored fixtures | design a respectful scheduled smoke check |
| Runtime observability | no documented health checklist for long-running monitor ops | failures harder to triage | add operator runbook and smoke checklist |
| Generated docs | `docs/generated/` is placeholder only | generated surfaces can rot later | define generation scripts once a generated artifact exists |
