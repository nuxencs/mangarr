# Asura Scans API Adapter

Status: completed on 2026-11-10.

## Problem

Since about 06:00 on 2026-11-10, every monitor check for Asura Scans fails with
`blocked by Cloudflare challenge: status code 403`. Cloudflare now serves a
managed challenge for all `asurascans.com` pages, including series pages and
`robots.txt`. Plain HTTP cannot pass the challenge, so the HTML adapter cannot
read any series.

## Diagnosis

- The failure reproduces locally with
  `mangarr download -s asurascans -m https://asurascans.com/comics/solo-max-level-newbie-bd5bdaf8 -L`,
  so it does not depend on the host address.
- The site frontend loads data from `https://api.asurascans.com`. That host
  does not serve the challenge. `/api/series/<slug>` returns the series,
  `/api/series/<slug>/chapters` returns all chapters with `is_premium`, and
  `/api/series/<slug>/chapters/<chapter slug>` returns the page image URLs.
- Images on `cdn.asurascans.com` download without a challenge.
- The API accepts a slug with an outdated hash suffix, such as `-bd5bdaf8`.
  The series endpoint redirects it to the current slug, and the chapter list
  answers it directly, so configured URLs keep working.

## Change

- `asurascans` reads the JSON API in place of the HTML pages. Colly and the HTML
  parsing code are removed from the adapter.
- Premium chapters are still filtered from discovery, and a chapter with no
  pages still fails as before.
- A regression test serves a Cloudflare challenge on `/comics/...` and the API
  on `/api/...`, and checks that discovery and page lookup succeed. A mock-server
  test also covers premium filtering.

## Verification

- `go test ./...`, `go test -race ./...`, `go build ./...`
- Live download of Solo Max-Level Newbie Ch. 284: 27 of 28 pages archived (the
  wide credits page is skipped as usual).

## Risks

- The API is not documented. Its shape can change without notice. The series
  endpoint has no `data` wrapper, but the chapter endpoints do.
  Update 2026-11-14: about 3.5% of monitor checks failed with `getting title
  for series`, because some API origins return the series in a `data` wrapper.
  The adapter now reads both shapes.
- The chapter list has no `is_locked` field. Discovery filters `is_premium`
  only. In all live samples, a chapter with a future `early_access_until` also
  had `is_premium=true`.
- The API gives a `null` title for some chapters, so those chapters have no
  title in their archive name.
