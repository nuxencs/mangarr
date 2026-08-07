# New User Onboarding

Verified against `README.md` and `docs/USAGE.md` on 2026-08-07.

## User Goal

Get one manga chapter downloaded quickly, with minimal setup and no code reading.

## Happy Path

1. install binary or pull Docker image
2. run `mangarr download --help`
3. choose a supported source
4. supply the source-specific manga identifier
5. save latest chapter to a local directory

## Friction Points

- source identifiers vary widely by provider
- some sources need full URLs, some IDs, some titles
- source-specific identifier requirements still add setup friction
- users need a clear path from quick start to deeper config/reference docs

## Acceptance Bar

- root README stays short and user-facing
- quick-start examples stay accurate
- source input expectations are easy to find
- deeper config and operator detail is reachable from README without reading Go code
