# Acceptance — VC-2 Cost accounting + model metadata (D9)

How a human can tell it worked.

## Token panel shows cost and cache hits

1. Start the split pair (`just run` + `cd ui; pnpm dev`) and open
   `http://127.0.0.1:3015`.
2. Have a conversation with a priced model (e.g. a known OpenAI model
   like gpt-4o) so usage events land in the Journal.
3. Open the token statistics panel (dashboard):
   - The overview now has a fifth card "Estimated cost" showing a `$` amount
     computed from the model's reference price.
   - The "Model distribution" table and the session tables (overview and detail)
     each have a "Cost" column.
   - The detail view shows "Cached tokens" next to reasoning tokens—for providers
     that report cache hits, this is nonzero.
4. Point a session at a model with no reference pricing (e.g. a custom
   gateway model id): its cost cells render as "—" with a tooltip
   "Price unknown for this model — cost excluded from totals", and the total only
   sums the
   priced rows. Cost for unpriced models never renders as $0.

## Image gating on text-only models

1. Configure a provider/model that is in the known reference table and is
   text-only (e.g. gpt-3.5-turbo).
2. Start a chat on that model and try to attach an image (paste or pick).
3. `turn/start` is rejected with an invalid-params error naming the model
   ("model ... does not support image attachments") — no run starts.
4. Point the session at an image-capable known model (gpt-4o) or an
   unknown custom-gateway model: attachments are accepted as before
   (unknown models keep the status-quo allow).

## Cached tokens flow end to end

1. With a provider that reports prompt cache hits (Anthropic-style or
   OpenAI prompt-caching), run a conversation.
2. `model.usage` journal payloads carry `cached_tokens`; the panel's
   cached-tokens metric reflects the sum for the selected period.

## Crush alignment

Behavior aligns with Crush's usage view (cost + cache surfaces on token
stats). No Crush code was used; all pricing/reference metadata and UI are
Vivy's own implementation (FSL-1.1-MIT compliance note, per track rules).
