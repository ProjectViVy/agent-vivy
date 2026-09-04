# VIVY CODE usage accounting integrity

## Changed

- Extended `model.usage` with additive provider/model/source attribution while preserving legacy Journal rows.
- Pinned main-run usage to its start route, carried cached tokens and the stable route through supervised child calls, and attributed summary primary/failover calls to the actual configured route.
- Restored those routes whenever approval or question recovery creates a fresh resume mapper, including after process restart, so resumed summary accounting cannot lose its provider/model identity.
- Made SQLite and Postgres projections prefer explicit usage attribution, fall back to the first deterministic `run.started`, and fail closed for ambiguous legacy summary rows.
- Unified sidebar and global token statistics on an all-priced contract. Mixed priced/unpriced requests expose zero placeholder cost with `cost_known=false`, never a partial total.
- Added cache-aware reference-cost math. Cached usage is unpriced unless a dedicated cache-read rate is known; impossible `cached > prompt` rows fail closed.
- Declared the current statistics scope as `chat_runs` in RPC, browser UI, and slash-command output. Automatic-title and manual session-compaction provider calls are explicitly excluded rather than silently implied to be counted.
- Added storage conformance CN-26 plus runtime/app/RPC tests for attribution, cached tokens, summary failover, resume/restart route recovery, duplicate start events, and strict pricing.

## Explicitly not done

- No provider cache price was guessed. `CachedInputPerMTokens` remains zero until a catalog rate is verified.
- Automatic-title and manual session-compaction calls are not retrofitted onto completed or synthetic runs. A future session-scoped auxiliary usage recorder is tracked in `TUI-USAGE-ACCOUNTING` if all-provider-call billing becomes the product requirement.
- No Studio source or tenant Journal was read or changed.

This is a focused source delivery, not a release; no release record is included.
