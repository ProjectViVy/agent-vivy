# Acceptance — VC-2 Anthropic 后端接线（eino-ext/claude）

How a human can tell it worked.

## Product view

1. Configure the Anthropic provider (Settings → Model, or the
   `ANTHROPIC_API_KEY` env var named by the bundle's `env_key`). The
   provider resolves instead of failing with "backend not wired yet" —
   that failure mode no longer exists anywhere.
2. Pick a Claude model (`claude-sonnet-4-5` etc.) and run a conversation:
   responses stream through the same Journal/approval/UI pipeline as
   OpenAI-compatible providers. Token/cost panels now show cost for
   Anthropic models (published reference prices; unknown models show
   "cost unknown" rather than $0).
3. First run after the last one (>5 min apart) shows cache-friendly usage:
   `stats/tokens` reports `cached_tokens` from Anthropic's
   `cache_read_input_tokens` via the component's usage mapping.
4. Removing the API key and running again fails with the actionable
   KeyMissingError message ("set the ANTHROPIC_API_KEY environment
   variable or configure it in Settings → Model"), never a raw SDK
   "missing api key" error, and never a silent env pickup.

## Live smoke step (needs a key)

With `ANTHROPIC_API_KEY` set and the anthropic bundle loaded, send one
message and confirm a run.completed event plus a visible assistant reply.
Optionally check the account console for cache_creation/cache_read token
lines on the second turn — evidence that the ephemeral breakpoints ride
system + tools + last message.

## Rollback

Single focused commit on `feat/vc1a-bash-tool`; revert it. The fixture
backend value reverts to the old (unwired) `vivy/anthropic` id, which the
pre-change binary understands; go.mod dependency additions disappear with
the revert. No schema or Journal migration involved.
