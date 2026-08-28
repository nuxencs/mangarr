# Comix Request Token Reverse Engineering

Research date: 2026-08-07

## Result

The current Comix `_` request token is fully reproducible without a browser. It is a deterministic encoding of the normalized API path and canonical query parameters. It does not include time, randomness, cookies, the user agent, request headers, or the HTTP method.

A pure Go implementation is feasible for frontend build `35595e3de3c99889c1aa70`. It would still be coupled to six hard-coded values in the current obfuscated bundle. Comix can rotate those values or change the algorithm in any frontend build.

This note describes the current behavior implemented by the Mangarr adapter.

## Sources

Only current first-party code and live Comix responses were used:

- [security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js), SHA-256 `27ca58cb8a3cb09968dead4edd726a847ee718e0678c865a927164952e32e76f`
- [API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js), SHA-256 `9d83388e88eff9d6b55709eadffaf151e4811907cd72f4cbb47b844ae61bec66`
- [reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js), SHA-256 `588fa873576087d8d71f1a5aac5f1dba4d6e309281097257f92a7f5369b14330`
- [live chapter-list endpoint](https://comix.to/api/v1/manga/pvry/chapters?page=1&limit=2)
- [live chapter endpoint](https://comix.to/api/v1/chapters/11169424)

The API client imports export `i` from the security bundle and calls it with the Axios instance. That export installs one request interceptor and one response interceptor. The request interceptor calls the internal `M2(url, params)` function and writes its result to `params._`. [API client bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/env-tjdqki-WSjnHwGH.js) [Security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

## Token input

The request interceptor only acts on `GET`. A missing method is treated as `GET`. `M2` then does this work:

1. Remove an `http://` or `https://` origin from the URL.
2. Remove the URL query at the first `?`.
3. Remove a leading `/api/v1`.
4. Continue only when the remaining path matches one of these expressions:
   - `^/manga(?:/|$)`
   - `^/chapters/[^/]+(?:\?|$)`
   - `^/groups/[0-9]+/chapters(?:\?|$)`
5. Recursively flatten the Axios `params` object.
6. Append `?` and the flattened parameters when the result is not empty.

Parameter canonicalization has these rules:

- Sort object keys at every level.
- Skip the `_` key.
- Skip `null` and `undefined` values.
- Write arrays in order with numeric bracket keys, such as `rating[0]=safe`.
- Write nested objects with bracket keys, such as `order[number]=desc`.
- Convert scalar values with JavaScript `String(value)`.
- Do not URL-encode keys or values.
- Join pairs with `&`.

For example:

```text
URL:    /api/v1/manga/pvry/chapters
params: {page: 1, limit: 20, order: {number: "desc"}}
input:  /manga/pvry/chapters?limit=20&order[number]=desc&page=1
```

The route and query are therefore both bound to the token. The method, origin, cookies, and headers are not part of the encoded input. The module also checks that it runs on a Comix host before it installs the interceptors. That host check is only a browser-side installation guard. It is not an input to the token. [Security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

## Token algorithm

Convert the canonical input to UTF-8. Pass the bytes through three chained substitution stages. Each stage uses this operation:

```text
previous = seed
for i from 0 through len(input)-1:
    output[i] = table[(input[i] XOR key[i mod len(key)] XOR previous) AND 255]
    previous = output[i]
```

Use these stages in order:

| Stage | Seed | Decoded table length | Decoded key length |
| --- | ---: | ---: | ---: |
| 1 | 189 | 256 | 24 |
| 2 | 133 | 256 | 24 |
| 3 | 32 | 256 | 32 |

The current Base64-encoded tables and keys are below. Standard Base64-decode each value before use. [Security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

```text
stage1.table = gbicCvAMzfcXEtGAyjvvhmb2yCWzWhjqcxXZ7ZhpzANOzoQLo3nuPZ2vK9dkb9hJExC0Vni/hdQBceI+mw611gkhQFjBuf4bJg1TxYqM+SL4YDqtwjxiGSdeH7so7Fn1HiRo37Z+RNvl44twXWVhomtMjw+8bemfmv9XEXr7mS82MxaCOJZRR0oHd9PLI5O+gyBGT6hcLoduNa7yCObVVCk3bFWsoD+xcqTrBcP6dNJN/NB1Br2QGhSN2snHAqeRNKVFQiyeAFLPSKGwY8aq9EPgsi17qd4ywPMxiH8w6N1qX1tLKtzhOeemHWeJQfFQ5H23q7qSlJUcjgTEl3x2/Q==
stage1.key   = rafYl4oSAKQX+GYoic9oW4iGwiYpZzs0

stage2.table = 2lQehmgyYFAoWUi0haazZqHy5zZ34NN+VzlfsoB2Y1yY0IuMLjgVcV2xt8t4moH+AP0NMJ5qekW7DFIHEWKkOgIBIMhDdA8lbM6iHKjDlq6IChpb3CnA9NmsvQW/afdt1SfJjTdwcvpKqunCJLxBFmXX9hecm6tGb+HRxD7BC3njoxPxgnX5pdKP1IMSkd4/O3NRfZSE6DVLG2s9uexaipA05cpJzE8Qkv/z5jzHAwlEWOLd3yxA+0cvVbpOoJPFGc8f1lb4vu2HUxjuuEwEQk0GsPCVnyKvfOoh9TG2YYmZLV4I67UU2NsrrakqZ47k/O+ne25/DjPGZCMdnZcmzQ==
stage2.key   = 2USAq+VTo5ht4bQn+K9DUcpUQRTtrB56

stage3.table = +mhJSFwzaV+PQPDyKp2scO/S9SdFsy/7e56UWT8XHbK3E2+19nEPwfwOgE9uVCaDtOAWTobCZX+cBCXlIbBqyDyQB1beKLspW6kGPhBCV9x0jf0KUeFhHjmlMf7qMFIB41PfDFprZ3bJiK4YxrZDv+K6dcwJmggVO8f5ktrXTM0cZL4fer0SpnkbvNajPbHxfuTz5lVEBarOI4rdc+2V6zTsjpfQYjgN1MMr6EvA6eehN6dQ1bgUogt9rZOBbQBeNnLYY00uZqSoJBnFi5gthCJsWF33ykosn9v/9KB8udMCz0YRYImrA4VHr5mMgpH4xDXLeEHRd5vZOiAalofuMg==
stage3.key   = yNHlokVEnuecesDrB/lDhVuUNiheWc3a47VtkwZ2ENg=
```

Encode the final bytes as unpadded Base64url. The token length is therefore the unpadded Base64 length of the UTF-8 input. There is no MAC, nonce, timestamp, or random salt.

The visible constant `n1PEbDBiipbJZvZc` belongs to other functions on the internal codec object. The request path calls codec method `D`, which uses the three stages above. That visible constant is not the request token key.

## Reproduction results

An independent implementation outside the page matched the bundle output exactly for:

- `/manga/pvry`
- `/manga/qqwrm`
- `/chapters/11170876`
- One Piece chapter-list pages 1 and 2
- a title search request

This current test vector is useful for regression checks:

```text
input = /manga/pvry/chapters?limit=20&order[number]=desc&page=1
token = IZ-P1pUtAjt3Oig5K1xNJ4A3XDWhN5iWaB2v65-B44CevAKBEF0OAOg-407yPS4VFzRxRsBwdg
```

Direct requests with independently generated tokens returned `200` for search, chapter list, and chapter detail without browser cookies. The chapter-list and chapter-detail responses had `x-enc: 1`. In a separate binding check, reusing a valid `limit=2` token with `limit=3` returned `403` and `{"message":"Invalid token."}`. [Live chapter-list endpoint](https://comix.to/api/v1/manga/pvry/chapters?page=1&limit=2) [Live chapter endpoint](https://comix.to/api/v1/chapters/11169424)

## Separate protection layers

The request token, API response decryption, and image descrambling are separate operations.

### Request token

The request interceptor generates `_` before Axios sends an eligible `GET`. Its only data input is the normalized path plus canonical parameters. This operation is pure byte processing and is practical in Go.

### API response decryption

The response interceptor checks for `x-enc: 1` and an object body with an `e` string. It Base64url-decodes `e`, reverses stages 3, 2, and 1, UTF-8-decodes the bytes, and parses JSON. The inverse of one stage is:

```text
previous = seed
for i from 0 through len(input)-1:
    output[i] = inverseTable[input[i]] XOR key[i mod len(key)] XOR previous
    previous = input[i]
```

After JSON parsing, the interceptor unwraps `{status: "ok", result: ...}` when present. A live two-item chapter-list response decrypted to an object with `items` and `meta`. This response codec uses the same hard-coded stage data, but it is not part of request token generation. [Security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

### Image descrambling

The reader imports security export `t` separately. It calls this function only for page entries marked as scrambled. The function fetches the image with CORS, omitted credentials, and an abort signal. It reads `X-Scramble-Hash`, `X-Scramble-Seed`, `X-Scramble-Grid`, and `X-Scramble-Algo`, builds a tile order with embedded WebAssembly, and draws reordered tiles to a canvas. This path uses browser image and canvas APIs. It is unrelated to `_` and API response decryption. [Reader bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/ReadPage-tjdqki-DcYDg-eV.js) [Security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tjdqki-DZP4UvtB.js)

The embedded WebAssembly exports `buildOrderV1` and `buildOrderV2`. Version 1 uses a Fisher-Yates shuffle driven by the standard 32-bit LCG constants `1664525` and `1013904223`. Version 2 uses Fisher-Yates with xorshift32 shifts `13`, `17`, and `5`, with the seed forced odd. The current `X-Scramble-Algo: 3` selects version 2. The hash-prefix table contains `03632 -> 58414` and `02900 -> 117532`. The effective seed is the response seed XOR the matching prefix.

### 2026-08-28 scramble-hash drift

The active frontend kept build directory `35595e3de3c99889c1aa70` but changed its generated bundle suffix from `tjdqki` to `tkempc`. Live images returned new opaque hashes `a8284` and `e05d1`. The active [security bundle](https://comix.to/assets/build/35595e3de3c99889c1aa70/dist/secure-tkempc-HovQ1K40.js) still contains the two legacy mappings above. Its lookup returns prefix zero when a hash is absent from that table. Both new hashes therefore use the response seed without an XOR prefix. Mangarr must preserve this fallback instead of rejecting unknown hash values.

The Go implementation reproduces the tile order and draws source tile `i` at destination tile `order[i]`. A live scrambled One Piece page reconstructed into a coherent page and passed a complete CLI archive smoke test.

## Go feasibility and gaps

Token generation, API response decryption, and image reconstruction can run in pure Go for this build. A browser and WebAssembly runtime are not required. An HTTP cookie jar was also not required by the tested logged-out API reads.

Remaining risks and gaps:

- The algorithm and all six values are private frontend implementation details. A build update can invalidate them without warning.
- The server-side acceptance window for tokens from an older frontend build is unknown.
- Mutation endpoints and signed-in endpoints were not tested.
- Comix has no public API contract or compatibility promise for this behavior.
