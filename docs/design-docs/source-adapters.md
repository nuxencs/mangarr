# Source Adapters

Verified against `internal/source/` on 2026-03-26.

All adapters implement `domain.Source`.

## Matrix

| Identifier | Input (`-m`) | Extra args | Validation highlights | Notes |
| --- | --- | --- | --- | --- |
| `tcbscans` | manga title | none | non-empty title | HTML scraping |
| `mangadex` | manga UUID | `-g` optional, `-l` optional | valid manga UUID | API-based |
| `mangaplus` | numeric title ID | none | strict numeric regex | protobuf API |
| `flamecomics` | full series URL | none | `https://flamecomics.xyz` prefix | HTML + embedded JSON |
| `asurascans` | full series URL | none | `https://asurascans.com/comics/...` | server-rendered HTML; filters locked early-access chapters from discovery |
| `cubari` | gist URL | `-g` required | valid URL + non-empty group | images resolved in payload |
| `weebcentral` | full series URL | none | `https://weebcentral.com` prefix | scraper + chapter image fragment fetch |
| `comix` | full title URL | `-g` optional | `https://comix.to/title` prefix | API-based |

## Rules

- validate early
- keep selectors and parsing local to the adapter
- prefer stable attributes or embedded data over brittle DOM traversal
- reuse `internal/sharedhttp/` and `internal/browser/` before adding new transport helpers
- when an adapter drifts, add a regression test around the parsing seam if practical

## Shared Behavior

- unknown source values fail fast in source selection
- `asurascans` removes chapters marked `is_locked=true` before returning the chapter map
- `weebcentral` fetches chapter images from the `/chapters/<id>/images` HTML fragment
- retry policy comes from `internal/sharedhttp/`
- manhwa/long-strip handling flows through `selectedManga.IsManhwa`
