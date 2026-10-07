# VCP-F2 — Prompt cache warming scheduler

## What landed

`runtime.cache_warming: off|streaming|idle` (default `streaming`) plus
`runtime.cache_warming_min_savings` (default 0.05 USD) drive a per-run
scheduler that refreshes Anthropic-family prompt caches before they
expire. Vendor data declares `supports_warming` +
`cache_lifetime_seconds` per model (set on the four modern Claude ids);
`claudeRef`'s `AutoCacheControl` already marks the cache breakpoints, so
a warm request is a minimal `Generate` carrying the run's stable prefix —
static instruction + selected tool schemas — plus a fixed user marker.
The reply is discarded.

## Behavior

- Streaming mode warms after every settled model call; idle mode arms a
  timer at 0.8 x declared lifetime, sliding on each settle.
- Savings gate: `last prompt tokens x input_per_mtok` must clear the
  floor; unpriced models fall back to a 2048-token proxy.
- Warm calls are unobserved (`withoutModelCallObserver` strips the
  run's observer binding): no model.request / model.usage /
  model.call.finished rows, no context or message pollution — the only
  trace is the diagnostic `cache.warmed` event
  (`warmed|skipped|failed`, with prompt + cache-write token accounting
  folded under `cache_write_tokens`).
- Scheduler cancels with the run; run-cancelled warms drop silently.

## Files

- `internal/runtime/cachewarmer.go` (new), `service.go`,
  `model_stream_observer.go`, `engine.go`, `payloads.go`
- `internal/config/config.go`, `internal/domain/{event,model}.go`
- `internal/provider/{vendor,catalog}.go`, `data/vendors.yaml`,
  `data/provider.schema.json`
