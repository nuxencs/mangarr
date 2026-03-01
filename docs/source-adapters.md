# Source Adapters

Adapters live in `internal/source/` and all implement `domain.Source`.

## Matrix

| Identifier | Input (`-m`) | Extra args | Validation highlights | Notes |
| --- | --- | --- | --- | --- |
| `tcbscans` | Manga title string | none | non-empty title | HTML scraping |
| `mangadex` | Manga UUID | `-g` group UUID optional, `-l` language | manga UUID required | API-based |
| `mangaplus` | 6-digit numeric title ID | none | strict numeric regex | protobuf API |
| `flamecomics` | Full series URL | none | URL must start with `https://flamecomics.xyz` | HTML + embedded JSON |
| `asurascans` | Full series URL | none | URL must start with `https://asuracomic.net` | Browser automation (Rod) |
| `cubari` | Gist URL | `-g` group required | valid URL + non-empty group | chapter images already resolved in manga payload |
| `weebcentral` | Full series URL | none | URL must start with `https://weebcentral.com` | mixed scraper + browser image extraction |
| `comix` | Full title URL | `-g` group optional | URL must start with `https://comix.to/title` | API-based |

## Selection Rules

- `download` command source selected from `-s`.
- `monitor` source selected from each `monitoredManga.<name>.source` entry.
- Unknown source values fail fast via `internal/source/source.go`.

## Browser-Based Sources

- `asurascans` and `weebcentral` require browser automation.
- Browser lifecycle handled centrally via `internal/browser.Manager`.
- CLI command creates one manager and reuses it across source calls.

## Manhwa Handling

- Some adapters mark manga as long-strip/manhwa (`IsManhwa = true`).
- Archive builder uses this flag to skip likely non-page assets (wide images).

## Shared HTTP Retry Policy

- Applies to source adapter requests and image downloads.
- Attempts: 3 total (2 retries), delay `1s`, max jitter `250ms`.
- Retries only transient failures: transport errors and `500/502/503/504`.
- Fails fast for `404/429/401/403/405` and other unexpected status codes.
