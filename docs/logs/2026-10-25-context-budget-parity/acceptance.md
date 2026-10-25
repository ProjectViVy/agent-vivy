# Acceptance: how to tell it worked

Owner-run; the numbers come from the journal and the run's own error text.

## A large child task now runs

Re-run the delegated task that failed with `266407 bytes; budget is 262144`:

- The child no longer terminates with `The child task context exceeds the
  configured limit.` at ~256 KiB.
- If it still fails on context, the message names the **new** ceiling
  (1 MiB ≈ 1,048,576 bytes) or the provider's own context-window error — which
  means the model window was reached, not our budget.

## Parent and child share one number

- `runtime.max_context_bytes` is the only ceiling: `/context` (or the Settings
  context meter) reports `limit_bytes` equal to that value, and the compaction
  trigger stays `min(model window × trigger_percent, limit_bytes/4)` tokens.
- A delegated child admitted with a large brief and large pending parent
  messages no longer drops them at 128 KiB; mailbox messages are included while
  they fit the same budget and are skipped (not fatal) once they do not.

## Known residual

Children still run without compaction. A delegated tool loop that grows past
the model's own window fails honestly at the provider instead of being folded;
`CHILD-CONTEXT-COMPACTION` in `docs/TODO.md` §0.1 owns that decision.

## Overriding

`runtime.max_context_bytes` stays operator-configurable in `config.yaml` (or
`settings.yaml` where the runtime section is exposed); raising it trades a
bigger request body for more room per run, and lowering it brings back the old
hard wall.
