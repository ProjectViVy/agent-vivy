# Summary — VC-2 成本核算 + 模型元数据 (D9)

Deliverable slice of the VIVY-CODE track (VC-2), decision D9: cost
accounting with model metadata that stays in sync with the provider/model
management surface ("逻辑与 web 端同步，不另起数据源") — one reference
table in the provider layer, resolved through the existing catalog, no
second store.

Crush alignment note: Crush is FSL-1.1-MIT — this slice aligns behavior
(the token panel gains cost + cache-hit surfaces like Crush's usage view)
with zero code copied; all implementation is Vivy's own.

## What changed

### Backend

- `internal/domain/model.go` — `ModelInfo` gains `InputPerMTokens`,
  `OutputPerMTokens` (USD per 1M tokens; zero = unknown, never free) and
  `SupportsImages`.
- `internal/provider/openai.go` — `knownOpenAIContextWindows` replaced by
  `knownOpenAIModels` (context window + reference in/out price per M +
  image support for the known OpenAI model set); `ModelInfo()` resolves
  all fields.
- `internal/runtime/payloads.go` + `mapper.go` — `model.usage` payload v1
  gains `cached_tokens` (from eino `schema.TokenUsage.PromptTokenDetails.
  CachedTokens`; `omitempty`). `schemas/events/payloads/model.usage.json`
  updated additively (optional field).
- `internal/storage/contracts.go` + sqlite/postgres usage projections —
  `UsageRow.CachedTokens` carried through from the payload.
- `internal/rpc/tokenstats.go` — `stats/tokens` snapshot gains
  `total_cached`, `total_cost_usd`, `cost_known` on the total and
  `cost_usd`/`cost_known` on model shares and sessions. Cost semantics:
  a row is priced iff the resolver returns nonzero in/out rates; unpriced
  rows are excluded from cost sums and reported `cost_known=false`
  (never read as $0); priced rows round to 4 decimals. The resolver is
  injected as `ModelMeta` in `ControlDeps` (nil resolver → nothing priced).
- `internal/rpc/control.go` — image-attachment gate at `turn/start`:
  rejects attachments only when the active model metadata is KNOWN
  (`ContextWindow > 0`) and `!SupportsImages`; unknown custom-gateway
  models keep the status-quo allow (gating on the zero-value default
  would break every custom gateway). This closes the VC-1g-2 carry-over.
- `internal/app/app.go` — wires `ModelMeta` from `catalog.ResolveModelInfo`.

### UI

- `ui/src/lib/api.ts` — `stats/tokens` TS types aligned with the wire
  (`total_cached`, `total_cost_usd`, `cost_known`; `cost_usd`/`cost_known`
  on model shares and sessions).
- `ui/src/components/demo/TokenStatsPanel.tsx` — overview gains a cost
  Metric card (5-card grid; em dash + "unpriced" tooltip when unknown);
  model distribution and session tables gain a Cost column (em dash when
  `cost_known=false`); detail view gains a cached-tokens Metric next to
  reasoning. Pricing never renders as $0 for unpriced models.
- `ui/src/i18n/en.ts` / `zh.ts` — new `token.unpriced` key; existing
  `token.cost` / `token.cacheTokens` keys reused.

## Explicitly not done

- Live-browser panel smoke with real usage rows — needs a provider key
  that does not exist in this environment (see verification.md).
- Anthropic provider metadata (reference prices) — openai set only; the
  resolution path is provider-agnostic and extends per provider later.
- `agent` sub-agent tool (D5), eino-ext/claude wiring, session auto-title —
  remaining VC-2 slices.
