# Acceptance — provider agentic migration

A human can tell the migration worked from these observations.

## 1. The build speaks one generation

- `rg -n 'ToolCallingChatModel' internal/runtime internal/provider` returns
  nothing: the seam returns `model.AgenticModel`, and request-time
  `model.WithTools` is how tools reach a call.
- `rg -n 'DEFERRED-INDEFINITE|openai-responses|ErrAdapterDeferred' internal sdk ui`
  returns nothing. The Responses family and the whole deferral vocabulary are
  gone from code, wire and UI.
- `internal/provider/adapters.go` lists exactly two families, and both are
  executable: `openai-completions` → `agenticopenai.NewChatModel`,
  `anthropic-messages` → `agenticclaude.New`.

## 2. Settings still offers a working model picker

1. Start the split dev pair (`just dev`) and open `http://127.0.0.1:3016`.
2. Settings → Model. The catalog lists the embedded vendors and their endpoint
   variants; every row is selectable. There is no "deferred" badge and no
   disabled row, because the wire no longer has an `executable`/`state` field.
3. Selecting a model persists `{provider, base_url, default_model}` with the
   adapter id as `provider`, and a normal turn completes.

## 3. Reasoning is a first-class block

Run a turn against a thinking-capable DeepSeek model and watch the run stream:
reasoning text arrives over the existing `model.reasoning_delta` event, then the
answer arrives as `model.delta`, and `model.usage` reports main-route token
counts. The journal schema did not change, so an existing client renders the
same way it did before the migration.

## 4. A paused run does not survive the generation change

Pause a run on an approval, then start a binary from before this change (or
rewrite a checkpoint envelope to omit `generation` — what the
`TestRecoverAcrossModelGenerationCancelsPendingApproval` test does). On restart
the paused run is closed with:

> Vivy was updated while this run was waiting, and a paused run cannot continue
> on a different engine version. Its pending request was cancelled. Please start
> the task again.

The pending approval, question or Plan review is cancelled, exactly as for an
engine-version change.

## 5. Fail-closed behaviour is unchanged where it matters

- A stored selection naming an adapter this build does not seal fails before any
  network call (`ErrAdapterUnknown` / missing Profile), never by falling back to
  another protocol.
- A missing API key still fails with the actionable `KeyMissingError` message.
- The Claude path still sends the protocol-required `max_tokens` floor and, for
  cache-enabled endpoints, an ephemeral prompt-cache breakpoint.
- The DeepSeek path still sends the `thinking` object and `reasoning_effort` for
  the thinking levels, and the sampling overrides declared per level.

## 6. Verification evidence

`docs/logs/2026-10-20-provider-agentic-migration/verification.md` records the
exact commands, counts and the three pre-existing `internal/app` failures plus
the three pre-existing UI suites and seven `tsc` errors that reproduce on the
untouched base commit.
