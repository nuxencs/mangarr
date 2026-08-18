# Source Adapters

Verified against `internal/source/` and all live sources on 2026-08-08.

All adapters implement the small `domain.Source` contract:

- `Discover` returns manga metadata and its chapter map.
- `Pages` returns an ordered page transport description for one chapter.
- adapters return values. They do not mutate caller-owned manga or chapter values.
- `source.Select` is the single registry used by download and monitor commands.

## Matrix

| Identifier | Input (`-m`) | Extra args | Validation highlights | Notes |
| --- | --- | --- | --- | --- |
| `tcbscans` | manga title | none | non-empty title | HTML scraping |
| `mangadex` | manga UUID | `-g` optional, `-l` optional | valid manga UUID | API-based |
| `mangaplus` | numeric title ID | none | strict numeric regex | mobile protobuf API; lazily registers a deterministic device secret |
| `flamecomics` | full series URL | none | `https://flamecomics.xyz` prefix | HTML + embedded JSON |
| `asurascans` | full series URL | none | `https://asurascans.com/comics/...` | server-rendered HTML; filters locked early-access chapters from discovery |
| `cubari` | gist URL | `-g` required | valid URL + non-empty group | images listed in the payload, or fetched from the `/proxy/...` path the gist points at |
| `weebcentral` | full series URL | none | `https://weebcentral.com` prefix | scraper + chapter image fragment fetch |
| `comix` | full manga URL | `-g` optional | `https://comix.to/title/...` prefix; numeric group when set | private API codec + referer-protected images + tile reconstruction |
| `atsumaru` | full manga URL | `-g` required | `https://atsu.moe/manga/...` prefix + non-empty scan ID | API-based |

## Rules

- validate early
- keep selectors and parsing local to the adapter
- prefer stable attributes or embedded data over brittle DOM traversal
- reuse `internal/sharedhttp/` before adding new transport helpers
- when an adapter drifts, add a regression test around the parsing seam if practical

## Shared Behavior

- unknown source values fail fast in source selection
- `mangaplus` uses the mobile API because the web protobuf endpoint rejects current unauthenticated access; chapter discovery reads the current `chapter_list_v2` field with legacy list fields kept as fallback
- `asurascans` removes chapters marked `is_locked=true` or `is_premium=true` before returning the chapter map
- `weebcentral` fetches chapter images from the `/chapters/<id>/images` HTML fragment
- `comix` implements frontend build `35595e3de3c99889c1aa70`; it generates request tokens, decodes encrypted API envelopes, sends image request headers, and reconstructs scrambled tile images
- `atsumaru` fetches chapter metadata from `/api/manga/info`, filters chapters by scan ID, and resolves relative page paths from `/api/read/chapter`
- source-specific image transforms use `domain.ImageProcessor`; the acquisition path owns transport and output while the source adapter owns the transform
- retry policy comes from `internal/sharedhttp/`
- Manga Plus request errors omit query strings so registration and device secrets do not enter logs
- HTTP and Colly-backed requests inherit caller cancellation
- fixture-backed parser flows cover every supported source
- manhwa/long-strip handling flows through `selectedManga.IsManhwa`
