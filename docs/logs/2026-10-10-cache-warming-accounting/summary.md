# Cache-warming accounting

Cache warming defaults to `off`, including an unset Service dependency. Explicit `streaming` warming now uses the observed call's exact leading system messages and bound tool schemas. Conversation tokens no longer justify warming a different, smaller prefix. The ineffective run-scoped `idle` timer is removed, and configuration validation rejects `idle`.

Each warm request uses the existing model-call observer and the owning run's budget: `model.request`, provider-reported `model.usage`, and mandatory `model.call.finished` with `source: maintenance`. The request's reply and marker do not enter conversation messages. Cache-write evidence survives normalization, Journal payloads, usage projection, and trajectory. Maintenance requests preserve pending chat deltas and the chat trajectory step.

Maintenance settles synchronously within the owning call, on a 60-second bounded context that cancels with the run. This avoids a new background owner and the previous race with the terminal Journal seal. Explicit warming adds a provider round trip at this boundary. Optional provider warm failures receive failed maintenance records/diagnostics; mandatory request, usage, or settlement persistence failures return through the owning observer's End. Integrated producer propagation uses the companion model-observer settlement change.

The retained `cache_warming_min_savings` setting gates estimated gross savings for one possible reuse of the exact prefix, using the shared four-byte token estimate and declared ordinary/cached input prices. It excludes warm cost and cannot establish net profitability. Missing prices conservatively skip; setting the floor to zero explicitly bypasses this estimate gate. Current provider metadata does not declare cached-read or cache-write rates, so a positive floor skips `unpriced_prefix`, and cache-write-bearing rows report unknown cost instead of charging writes at the ordinary input rate.

`cache.warmed` joins its maintenance call identity and records a digest of the exact system/tools prefix, estimated prefix tokens, and actual reported prompt/output/cache-read/cache-write counters. Deterministic integration tests compare two identical warm prefixes: a scripted creation/write attempt and a scripted cache-read hit. These are accounting and prefix-identity checks, not live-provider measurements. No live billed cost, hit rate, cache retention, or net saving has been measured. A full chat prompt's cached count cannot be assigned to this smaller prefix without additional provider evidence.

## Eino capability check

Inspected pinned Eino v0.9.13 `components/model` Generate/options, `schema.ConcatMessages` usage merging, callbacks/stream-copy semantics, and EinoExt Claude v0.1.25 `convTokenUsage` / `GetCacheCreationInputTokens`. Claude prompt totals include ordinary input, cache reads, and cache creation; creation is a reported subset, not additional prompt tokens. The existing runtime observer supplies the mandatory Journal preflight/settlement behavior callbacks cannot enforce. This change reuses that seam, the native model options, and the shared token estimator; it adds no alternate runtime, provider, database, background service, or rate catalog. Replace the tiny observer tracker if Eino supplies the same mandatory persistence error classification.

No release artifact is built in this lane; integration and the full product gate belong to the parent delivery.

## Review follow-up

Optional maintenance denied by its Begin model-call/event budget is now `skipped` with `budget_exhausted`, without provider invocation or failure of the already successful main call. This exception applies only before admission: paid-call usage budget failure, Journal admission failure, and mandatory settlement persistence failure still return through the owning End. A real single-call run at `MaxModelCalls: 1` remains successful with zero paid warm calls.

Missing cache-write presence also keeps maintenance and catalog-declared warming-capable rows unpriced even if their other reference rates are known. Route aggregates combine cache-write presence conservatively across every included row, so source ordering cannot conceal missing creation evidence. An explicit reported zero-write bucket can use known ordinary/read/output rates; absent evidence is never inferred to be zero.
