# Bulk download rate-limit reliability

## Problem and scope

Improve `download --all` reliability without changing chapter selection, force
re-download policy, config selection, or source parsing. Use local HTTP fixtures
and temporary output directories only; no provider stress or real library writes.

## Diagnosis

- Actual path: Cobra `download --all` -> `source.Select` -> `Discover` -> selected
  chapters -> `acquire.Chapter` -> source pages -> `download.Chapter` ->
  `fetchWithRetry` -> shared HTTP status classification.
- Reproduction: `go test ./cmd -run '^TestDownloadRateLimit$' -count=1 -v`.
  A local Cubari-shaped gist contains two chapters with one PNG each. Chapter 1's
  image returns 429 once with `Retry-After: 0`, then 200. Chapter 2 returns 200.
- Observed baseline: `downloaded=1 skipped=0 failed=1`, `Failed chapters: 1`,
  non-nil CLI error, and only one request for the throttled image. Repeated twice.
- Successful controls: all-200 downloads both archives; `--latest` selects only
  chapter 2 and succeeds against the same fixture. Changing only 429 to transient
  503 downloads both archives using two attempts for chapter 1.
- Initiating trigger: a selected image request receives HTTP 429.
- Masking conditions: latest-only selection never touches the throttled image;
  an existing archive would skip image acquisition; a provider returning 503
  instead enters the existing retry path. Bulk's 10 chapter slots, each with up
  to 10 image jobs, increase exposure but are not necessary for this failure.
- Visible failure: the entire affected chapter fails and is absent from output,
  even when the next request would succeed; the command returns failure.

Ranked hypotheses and predictions:

1. Terminal 429 classification prevents recovery: removing only that terminal
   classification should make the CLI fixture pass without changing scheduling.
2. Concurrent chapter requests are necessary: selecting just chapter 1 should
   then succeed. Disconfirmed: `--first` also fails after one 429 before the fix.
   The fixture does not model contention or establish a safe live-provider
   concurrency threshold.
3. Lost response headers prevent guided retry timing: a response carrying
   `Retry-After` should not be retried before that deadline. Test with fake time.

History: `bae1eae` made 429 terminal in 2024; `9003fa4` preserved this policy and
added explicit coverage in the 2026 shared retry refactor. `f9027c5` removed the
cooldown after the final batch, but a two-chapter run never used that cooldown to
pace its initial requests. There is no evidence that recent change caused the
reported live throttling.

Evidence limitation: no affected provider, exact invocation, log, retry headers,
or live request trace was supplied. This is a representative end-to-end failure,
not a reproduction of the unspecified provider. It supports a shared bounded
retry fix; it does not justify provider-specific pacing or parser changes.

## Plan

- [x] Establish deterministic red-capable CLI fixture and successful controls.
- [x] Test minimal code counterfactual and disconfirming single-chapter case.
- [x] Add bounded 429 retries and honor Retry-After without premature retries.
- [x] Cover guidance parsing, exhausted budget, permanent failures and cancellation.
- [x] Update usage/reliability docs, run full gate, and record remaining limits.

## Decisions

Reuse the existing shared retry boundary rather than retrying whole chapters or
adding a second retry layer. Three attempts total retain the existing one-/two-
second exponential backoff plus jitter. Valid Retry-After guidance extends that
wait. Guidance above five minutes fails explicitly instead of truncating it and
retrying early; oversized numeric guidance cannot overflow into a short delay.
Final errors retain HTTP status plus image/chapter context. There are no new
CLI/config options or changes to chapter selection, concurrency, or monitor
scheduling. Direct HTTP monitor requests inherit only the necessary shared fix.

## Verification

- Baseline CLI fixture: controls pass; transient 429 fails as described above.
- Minimal causal counterfactual: removing only `retry.Unrecoverable` from the 429
  status branch makes both `--all` and `--first` succeed. No scheduling change is
  needed for this failure; the transient-503 successful path already uses that
  retry boundary.
- Final CLI regression: recovery produces both valid CBZ archives. Persistent
  429 stops at three requests, preserves the other chapter, publishes no partial
  archive, and returns `failed to download chapters: 1`. A 404 still makes just
  one request and returns the same useful chapter summary.
- Fake-time transport fixtures exercise actual `ExecRequest` plus `RetryOptions`:
  delta seconds, HTTP dates, updated guidance, missing/malformed/expired values,
  maximum and oversized waits, bounded exhaustion, response closure, permanent
  failures, pre-cancellation, and cancellation during a guided wait.
- `go test ./cmd ./internal/sharedhttp -count=1`: passed.
- `go test ./...`: passed.
- `go test -race ./...`: passed.
- `go build ./...`: passed.
- `go vet ./...`: passed.
- No live source was exercised: no affected source or trace was supplied, and
  manufacturing live throttling would be inappropriate. No browser tooling was
  required or installed.

## Follow-up risks

Colly discovery/page scraping does not use `ExecRequest`; this shared fix covers
all image requests and existing direct-HTTP adapters, not every scraper request.
Live provider traces would be needed before choosing provider-specific limits.
No host-wide cooldown is introduced, so sustained limits can still exhaust the
bounded budget. Existing archives remain the resume mechanism. A future report
showing scraper-only throttling or nonstandard retry headers could change scope;
the current evidence establishes the shared failure independently of those gaps.
