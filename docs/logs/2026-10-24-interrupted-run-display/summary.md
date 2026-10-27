# Interrupted run display — summary

## Decision

Owner decision (2026-10-24): a cancelled or failed run keeps its partial
text/reasoning on screen and is marked **已中断** (`Interrupted`). Do not hide
the output, and do not fabricate a completed answer.

## What was wrong

A cancelled/failed run never journals `model.completed`, so the kernel never
persists its trailing buffer. The browser, however, folds raw Journal deltas and
flushed the last uncommitted text/reasoning buffer as a normal settled bubble:

- `foldRunEvents` decided `streaming`/`running` from the caller's `active` flag
  only, so every settled run's trailing buffer became a "final" answer.
- The live run lost `active` the moment `run.cancelled`/`run.failed` arrived,
  producing the same flush.
- A stale `running` status could also leave unfinished tool calls looking
  running forever after the log had already settled.

This was display-only: no `messages` row exists for the trailing buffer. The
persisted variant was repaired separately by migration 048
(`docs/logs/2026-10-24-subagent-transcript-leak/`).

## What changed (UI only)

`ui/src/lib/run-rows.ts` — the fold is now terminal-aware:

- It records the last terminal event in the log: `run.completed`, `run.failed`,
  `run.cancelled`, and the child-session equivalents `child.completed`,
  `child.failed`, `child.cancelled`.
- `active` becomes `options.active === true && !sequenceEnded`, so a terminal
  event in the log beats a stale status for one render after settlement.
- A trailing text/reasoning buffer that exists at flush time for a
  cancelled/failed run is emitted with `interrupted: true`; the content is kept
  and `streaming`/`running` are cleared.
- Unfinished tool calls are closed (`stopped`) whenever the log is settled
  (`!active`), not only when the caller already says inactive.
- Completed runs, still-active logs, and logs with **no** terminal event (a
  background run still streaming, or a truncated/pruned history) stay unmarked:
  the fold cannot honestly call those interrupted.
- `TranscriptRow` carries `interrupted` onto the assistant transcript row,
  including the synthetic row that is not backed by a persisted message.

Rendering:

- `MessageBubble` takes an optional `interrupted` prop, renders a muted
  `已中断` chip before the timestamp, and sets `data-interrupted="true"` on the
  article. The body, copy, regenerate, rewind and fork actions are unchanged.
- `ReasoningRow` takes `interrupted`, sets `data-state="interrupted"` and shows
  the same chip after the `思考过程` label; `running` still wins.
- `ChatView` passes the fold result to both components.
- i18n adds `chat.interrupted` (`已中断` / `Interrupted`).

No store change: the terminal event already reaches `runEvents` and every
`run/log` replay, and the fold scans the whole log, so event order does not
matter.

## Explicitly not done

- No kernel, Journal, storage, migration, RPC, or persisted-projection change.
- No UI change for the case "log has no terminal event": that shape stays
  unmarked by design.
- Known limitation (recorded, not fixed): an interrupted assistant row is
  synthetic, so its `message.id` is not a persisted message and rewind/fork on
  that row would fail in the kernel. The same row already offered those actions
  when it was rendered as a final bubble, so this delivery does not change that
  hazard.

## Eino capability check

No Eino surface applies. This is a browser-side fold over Journal events the
kernel already emits; no model, tool, orchestration, context, or storage path
changed.

## Files touched

- `ui/src/lib/run-rows.ts`, `ui/src/lib/run-rows.test.ts`
- `ui/src/components/chat/MessageBubble.tsx`, `MessageBubble.test.tsx`
- `ui/src/components/chat/ReasoningRow.tsx`, `ReasoningRow.test.tsx`
- `ui/src/components/chat/ChatView.tsx`
- `ui/src/i18n/zh.ts`, `ui/src/i18n/en.ts`
- `docs/TODO.md`, `docs/COMPLETE.MD`
- `docs/logs/2026-10-24-interrupted-run-display/` (this record)
