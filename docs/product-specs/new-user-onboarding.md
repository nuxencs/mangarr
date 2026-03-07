# New User Onboarding

Verified against `README.md` on 2026-03-07.

## User Goal

Get one manga chapter downloaded quickly, with minimal setup and no code reading.

## Happy Path

1. install binary or pull Docker image
2. run `mangarr download --help`
3. choose a supported source
4. supply source-specific manga identifier
5. save latest chapter to a local directory

## Friction Points

- source identifiers vary widely by provider
- some sources need full URLs, some IDs, some titles
- browser-backed sources add runtime dependency complexity

## Acceptance Bar

- README quick-start examples stay accurate
- source input expectations are easy to find
- first successful `download` does not require reading Go code
