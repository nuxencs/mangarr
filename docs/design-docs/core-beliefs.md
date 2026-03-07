# Core Beliefs

Verified on 2026-03-07.

## Agent-First Principles

1. Progressive disclosure beats giant setup docs. Start with a small stable map, then branch.
2. Docs are part of the product surface for maintainers. Stale docs are defects.
3. Plans are first-class artifacts for non-trivial work. Important decisions should survive the thread.
4. Root-cause fixes over patches. If drift or duplication is visible in the touched area, reduce it.
5. Mechanical sympathy matters. Keep concurrency, retries, and browser automation explicit.
6. Prefer shared seams for transport, archive creation, and config handling. Keep source-specific hacks contained.
7. Verification is mandatory. Narrow check while iterating; full gate before handoff.

## Repo Biases

- CLI-first, no web app
- network-heavy integrations with unstable upstreams
- maintainability over abstraction density
- explicit operational notes for long-running monitor mode
