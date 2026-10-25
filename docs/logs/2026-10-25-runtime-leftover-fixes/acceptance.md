# Acceptance: how to tell it worked

Owner-run; evidence comes from the Run Inspector and the Settings save path.

## Retries survive a settings change

- Change a model/tool/MCP/compaction setting that rebuilds the engine, then
  trigger a run that hits a transient provider failure (a bare 400/2013 or a
  tool-result-order rejection).
- The Run Inspector still shows one `auto_retry.started` / `auto_retry.finished`
  pair. Before this fix the rebuilt engine had no decider and the failure went
  straight to `run.failed` until the process restarted.

## One recovery per run

- A run that already recovered from a context overflow publishes exactly one
  `auto_retry.started`, one `context.compacted`, and one
  `auto_retry.finished{success:true}`.
- A later 400 (2013 or tool-order) in the same run fails honestly: no second
  `auto_retry.started`, no second compaction.

## Compaction never absorbs a child transcript

- Delegate a task, let the child finish, then run `/compact` (or wait for
  automatic compaction).
- The durable summary contains the parent's turns only; child output stays
  available through the `agent` tool card and is not duplicated into session
  history. `ContextStatus` / the context meter likewise counts only the parent
  feed.

## Overflow-shaped 2013 is compacted, not replayed

- A primary run rejected with a 2013 whose text names a context overflow (for
  example `prompt is too long (2013)`) shows `context.compacted` plus one
  retried model call, instead of an identical replay.
- The same rejection on a delegated child fails honestly, because child runs
  still do not compact.

## Residual

Children still run without compaction; a delegated tool loop that grows past
the model's own window fails at the provider. `CHILD-CONTEXT-COMPACTION` in
`docs/TODO.md` §0.1 remains the owner of that decision.
