# Acceptance

1. Complete main and child model calls in one session and inspect `stats/tokens`: both token totals are counted, child cached tokens survive, and provider/model grouping uses each call's real route.
2. Trigger compaction with a configured summary model and then its failover path; the primary usage belongs to the summary model and failover usage belongs to the main model.
3. Mix priced and unpriced routes in one total, model group, or session. Confirm `cost_known=false` and cost `0`; no partial sum may be presented as the complete cost.
4. Supply cached usage without a verified cache-read rate, or with cached tokens greater than prompt tokens. Confirm cost is unknown. With a dedicated non-zero cache rate, confirm uncached input, cached input, and output are priced separately.
5. Replay legacy usage without attribution and confirm it uses the first `run.started` route exactly once even if a malformed duplicate start event exists.
6. Confirm browser statistics and `/stats` identify the scope as chat runs and explicitly exclude automatic-title/manual-compaction calls.
