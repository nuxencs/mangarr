# Comix API Research

Research date: 2026-08-07

## Decision

Restore the Comix adapter with an explicit compatibility warning.

Comix has a usable private web API, but it does not expose a stable public API. Mangarr now implements the request token, encrypted response envelope, image referer, and tile reconstruction used by frontend build `35595e3de3c99889c1aa70`. This works without a browser, but it remains coupled to private implementation details that Comix can change without notice.

## Scope and evidence

This note uses first-party sources only:

- live Comix pages and API responses
- the current Comix frontend bundles
- browser-observed requests made by the Comix frontend
- Comix `robots.txt`, upload rules, credits, and contact pages
- Mangarr source, domain, browser, and downloader contracts

The inspected frontend build identifier was `35595e3de3c99889c1aa70`. Bundle names and behavior can change without notice. Comix did not publish source maps at the adjacent `.map` URL during this research.

## Endpoint summary

The current frontend configures an Axios client with base URL `/api/v1`, a 15-second timeout, `withCredentials: true`, `Accept: application/json`, and `X-Requested-With: XMLHttpRequest`. It installs a request interceptor from an obfuscated security module. [Current API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js) [Current security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

| Operation | Current request | Access and response behavior |
| --- | --- | --- |
| Search/list titles | `GET /api/v1/manga?keyword={text}&limit={n}` | Protected by `_` token. The home search also sends repeated `content_rating[]` values. |
| Title metadata | `GET /api/v1/manga/{hid}` | Protected by `_` token. The public title HTML also contains metadata in `#initial-data`. |
| Title groups | `GET /api/v1/manga/{hid}/groups` | Declared by the current client. The title HTML also includes the group list in `#initial-data`. |
| Chapter list | `GET /api/v1/manga/{hid}/chapters` | Protected by `_` token. Supports page, limit, ordering, group, number, and reader-centered queries. |
| Chapter detail and pages | `GET /api/v1/chapters/{numericChapterID}` | Protected by `_` token. The response supplies chapter metadata and page data. |
| Anonymous user state | `GET /api/v1/user` | Returned `200` with `{"status":"ok","result":null}` and set a two-hour anonymous session cookie in a direct logged-out request. |

The client endpoint declarations are in the [current API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js). The reader calls `/chapters/{id}` and normalizes the two page payload forms in the [current reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js).

The old Mangarr `/api/v2` paths now return `404`. Direct calls to the current title, search, chapter-list, and chapter-detail endpoints without `_` return `403` and `{"message":"Missing token."}`. Browser calls from the site add `_={opaque-value}` and return `200`. Replaying a browser URL worked, but changing its `page` parameter while keeping the token returned `403` and `{"message":"Invalid token."}`. The token is therefore bound to at least the route and query. [Example title endpoint](https://comix.to/api/v1/manga/pvry) [Example chapter-list endpoint](https://comix.to/api/v1/manga/pvry/chapters?page=1&limit=2) [Example chapter endpoint](https://comix.to/api/v1/chapters/11169424)

Successful protected responses observed through the browser used `Content-Type: application/json`, `x-enc: 1`, and an encrypted body shaped as `{"e":"<opaque data>"}`. The frontend response interceptor returns decoded application objects to React. [Current API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js) [Current security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

## Authentication, cookies, and headers

Reader access did not require a signed-in account. The logged-out frontend fetched title, chapter-list, and chapter-detail data successfully. It still uses credentialed same-origin requests and establishes an anonymous `session` cookie. An implementation must therefore preserve a cookie jar even if it does not support user login. [Comix home](https://comix.to/) [Current API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js)

Observed API request headers:

```http
Accept: application/json
Referer: https://comix.to/title/<hid>-<slug>
User-Agent: <browser user agent>
X-Requested-With: XMLHttpRequest
```

There was no `Authorization` header. The protected requests placed the opaque value in the `_` query parameter.

The site supports account login and registration, and those forms use Cloudflare Turnstile. This authentication is for user functions, not basic reader access. [Comix home](https://comix.to/) [Current main bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/main-tjdqki-DltojfL5.js)

## Search and title metadata

The home search issued this request while logged out:

```http
GET /api/v1/manga?keyword=One+Piece&limit=6&content_rating[]=safe&content_rating[]=suggestive&_=<redacted>
```

The frontend also uses the same endpoint for browse results with `page`, `limit`, type, status, tag, and order parameters. List responses expose an `items` collection and pagination metadata. The current UI reads `total`, `perPage`, `page`, `lastPage`, `from`, `to`, `hasNext`, and `hasPrev`. [Comix home](https://comix.to/) [Current main bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/main-tjdqki-DltojfL5.js) [Current API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js)

A title URL has this form:

```text
/title/{hid}-{slug}
```

For example, the One Piece page uses hash ID `pvry` and numeric internal ID `366`. Its server-rendered `#initial-data` object includes `id`, `hid`, `title`, alternative titles, type, status, language, poster URLs, latest and final chapter values, chapter availability, dates, synopsis, content rating, external links, and first/latest chapter URLs. It also includes the title's scanlation groups. [Example title page](https://comix.to/title/pvry-one-piece)

Use `hid` as `domain.Manga.ID`. It is the API and URL identifier. Treat the numeric manga ID as secondary metadata. The human-readable slug is not an identifier and can change when the title changes.

## Chapter listing and pagination

The title page issued this request:

```http
GET /api/v1/manga/pvry/chapters?page=1&limit=20&order[number]=desc&_=<redacted>
```

The UI also sends these optional parameters:

- `group_id={numericGroupID}`
- `number={chapterNumber}`
- `around={numericChapterID}` for a reader-centered page
- `order[number]=asc|desc`

The UI reads `items` and pagination metadata, including `page` and `lastPage`. It uses page numbers, not cursors. [Example title page](https://comix.to/title/pvry-one-piece) [Current main bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/main-tjdqki-DltojfL5.js) [Current reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js)

Chapter links use this form:

```text
/title/{hid}-{slug}/{numericChapterID}-chapter-{chapterNumber}
```

The numeric chapter ID is stable enough for the chapter-detail endpoint. A title can contain duplicate chapter numbers from different groups. For example, the live One Piece list contained two chapter 1189 entries with different IDs and groups. Mangarr's `map[domain.ChapterNumber]domain.Chapter` cannot represent both. A Comix adapter must require a group ID or apply a documented deterministic preference before it fills that map. [Example title page](https://comix.to/title/pvry-one-piece)

## Chapter pages and image retrieval

The chapter HTML supplies only `mangaId`, `mangaHid`, `chapterId`, and `chapterNumber` in its server-rendered `#initial-data`. It does not supply page URLs. The reader fetches them from `/api/v1/chapters/{numericChapterID}` after startup. [Example reader page](https://comix.to/title/pvry-one-piece/11169424-chapter-1190) [Current reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js)

The frontend supports two decoded page forms:

```json
{"pages":[{"url":"https://<image-host>/<opaque-path>","width":800,"height":1200}]}
```

or:

```json
{
  "pages": {
    "baseUrl": "https://<image-host>",
    "items": [
      {"url":"/<opaque-path>","width":800,"height":1200,"s":1}
    ]
  }
}
```

For the compact form, concatenate `baseUrl` and each item URL in listed order. `s: 1` marks a scrambled image. The reader passes such images to an obfuscated function and renders the result to a canvas. It uses a normal `<img>` only when the scramble flag is absent. [Current reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js) [Current security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

One current chapter returned 16 page entries and used normal images. Its page host rejected a Mangarr-like request without a referer with `403`, then returned `200 image/webp` when the request used `Referer: https://comix.to/`. The successful response allowed byte ranges and exposed scramble-related header names through CORS. The page host is external to `comix.to`, so it is operational evidence, not a first-party policy source. The first-party reader generated these URLs and sent the root Comix referer. [Example reader page](https://comix.to/title/pvry-one-piece/11169424-chapter-1190) [Current reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js)

## Rate limits and anti-bot behavior

Comix is served through Cloudflare. Pages include Cloudflare challenge code, and account forms use Turnstile. Protected API calls require the obfuscated `_` token. These are active anti-automation controls. [Comix home](https://comix.to/) [Current main bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/main-tjdqki-DltojfL5.js) [Current security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

No rate-limit policy or quota was found. Sample `200`, `403`, and `404` responses did not include standard limit, remaining, reset, or `Retry-After` headers. The frontend has a generic message for HTTP `429`, but it does not document when Comix returns it. No load test was run. [Current main bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/main-tjdqki-DltojfL5.js)

An adapter must use low concurrency, cache title and chapter results, stop on `403` and `429`, and avoid retry storms. Mangarr already treats `403` and `429` as unrecoverable in `internal/sharedhttp`.

## Legal and operational risks

Comix does not expose API documentation or an API license in the current site navigation. The `/terms` and `/privacy` routes returned `404` during this research. The visible policy-related pages were Contact, Credits, Upload Request, and Upload Rules. [Comix home](https://comix.to/) [Contact](https://comix.to/contact) [Credits](https://comix.to/credits) [Upload rules](https://comix.to/upload-rules)

`robots.txt` generally allows crawling, but it blocks several named AI crawlers. Its content signal permits search indexing and reference use, and rejects AI training. This file is not an API license or permission to redistribute chapter images. [Comix robots.txt](https://comix.to/robots.txt)

The upload rules permit official releases and do not prohibit uploads of material that is restricted elsewhere. The credits page names other aggregators and scanlation groups as content sources. This creates material copyright, provenance, and availability risk for a downloader integration. Mangarr keeps its personal-use warning and must not imply that Comix grants redistribution rights. Distribution decisions can still require legal review. [Upload rules](https://comix.to/upload-rules) [Credits](https://comix.to/credits)

The technical controls are also an operational warning. Reproducing the obfuscated token, decryptor, or descrambler would couple Mangarr to code that Comix can change on every build. A browser-backed implementation would add browser startup cost, larger failure scope, and ongoing maintenance for Cloudflare, lazy loading, and canvas output.

## Mangarr adapter mapping

The implementation uses this mapping:

| Mangarr operation | Comix mapping | Required handling |
| --- | --- | --- |
| `ValidateInput` | Full `https://comix.to/title/{hid}-{slug}` URL | Validate scheme and host. Extract `hid`. Reject URLs without a title path. |
| `GetManga` | Title HTML `#initial-data` or `GET /api/v1/manga/{hid}` | Map `hid` to `Manga.ID`, sanitized title to `Manga.Title`, and `manhwa` or `manhua` type to `Manga.IsManhwa`. |
| `GetChapters` | `GET /api/v1/manga/{hid}/chapters` | Follow `page` through `lastPage`. Filter by configured numeric group ID before writing the chapter map. Map numeric chapter ID, number, title, and reader URL. |
| `GetImageURLs` | `GET /api/v1/chapters/{numericChapterID}` | Preserve page order. Normalize direct and compact page formats. Carry referer requirements and scramble metadata. |
| Image download | External page URL | Send the required Comix referer. Apply the header-driven tile transform for scrambled pages. |

Current implementation status:

- `internal/source/comix_protocol.go` generates `_` and decodes encrypted API envelopes.
- `internal/source/comix_image.go` implements both frontend tile-order algorithms and writes reconstructed pages as PNG.
- `domain.ImageInfo` carries per-image headers and an optional source-owned processor.
- `internal/download` applies request headers and processors without importing Comix behavior.
- `domain.Manga.Chapters` still cannot represent duplicate chapter numbers. An optional numeric group ID selects one group; without it, the first chapter in server order wins.

## Maintenance conditions

Keep Comix enabled only while these conditions remain true:

1. Fixed vectors continue to match the current frontend request and image algorithms.
2. The API response shapes remain compatible with the adapter fixtures.
3. `403` and `429` remain unrecoverable, so protocol drift and rate limits do not cause retry storms.
4. A live smoke test can fetch metadata, paginate chapters, download every page, reconstruct scrambled pages, and open the final archive.
5. User documentation continues to warn that Comix uses a private protocol and can require prompt adapter updates.
