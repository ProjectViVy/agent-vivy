# Thinking row: one live renderer, left-to-right summary

Date: 2026-09-19
Owner: Vivy kernel UI (`ui/`).
Trigger: two display regressions reported against the page after
`547e656 feat(ui): show tool calls and reasoning as DSH-style transcript rows`:
while VIVY thinks, two thinking bubbles can appear at once, and the collapsed
(simplified) thinking summary printed right-to-left instead of left-to-right.

## Root cause

Phase 1 added an event-folded transcript renderer but left the older live
fallback bubble in place, so one live run had two renderers:

1. `ChatView` folded the live run's events (`runEvents`) into transcript rows and
   rendered `ReasoningRow` + assistant bubbles from them.
2. `ChatView` also kept `streamMessage` (`streamingText || streamingReasoning`)
   and passed `reasoning` into `MessageBubble`, which rendered a second thinking
   disclosure (`<details>思考过程（进行中）`) inside a card bubble.

Both read the same source: the store appends `model.delta` /
`model.reasoning_delta` to `runEvents` **and** to `streamingText` /
`streamingReasoning`, so whenever the fallback bubble existed the transcript was
already drawing the same content. During reasoning that showed two thinking
affordances (the row plus a `…` bubble carrying the second disclosure); once text
streamed, the same text could be drawn twice.

The second report is the reasoning row's own layout: the running summary was
wrapped in `flex … justify-end overflow-hidden` (copied from DSH's
`data-follow-end`), so while thinking the summary was pinned to the row's right
edge and grew leftward; the settled summary was already left-aligned.

## What changed

1. `ui/src/components/chat/ChatView.tsx`: the event-folded transcript is the
   single live renderer. Removed the `streamingText` / `streamingReasoning`
   selectors, the `streamMessage` constant, its render, and the empty-state
   dependence on it. Projection-only rendering for runs whose events are not
   loaded is unchanged (`run-rows.ts` fallback).
2. `ui/src/components/chat/MessageBubble.tsx`: removed the `reasoning` prop and
   the `<details>` thinking disclosure — the second renderer of the same fact.
   `reasoning=` had exactly one call site (the removed line), so this is a
   deletion, not a dormant prop. The empty-shell guard is now
   `content.trim() === '' && !streaming`.
3. `ui/src/components/chat/ReasoningRow.tsx`: running and settled summaries share
   one element — `min-w-0 flex-1 truncate` (left-aligned, ellipsis at the right
   end) — so the summary always grows left to right. The running-only
   `aria-live` announcement, `data-state` and the sweep overlay are unchanged.

## Files

- `ui/src/components/chat/ChatView.tsx`
- `ui/src/components/chat/ChatView.test.tsx` (new, happy-dom)
- `ui/src/components/chat/MessageBubble.tsx`
- `ui/src/components/chat/ReasoningRow.tsx`, `ReasoningRow.test.tsx`

## Explicitly not done

- No kernel, wire, Journal, projection or `run-rows.ts` folding change; the
  transcript still folds the events the `run/log` RPC already serves.
- `chat.thinkingStreaming` stays in the `en`/`zh` catalogs after losing its last
  consumer: the cross-face gate requires declared keys to be present, not used,
  and the dictionaries belong to another active lane's diff.
- `store.streamingText` / `store.streamingReasoning` (and `replay`, whose only
  caller was the removed line) are now unread by the built-in Face UI. They stay
  in `ui/src/lib/store.ts` and in the published `FaceStoreState`
  (`sdk/ui/src/module.ts`) for now — removing members from the published Face
  contract is its own compatibility-recorded change and that file is modified by
  the resident approval-timeout lane. Recorded as `LIVE-TAIL-DERIVED-DUP` in
  `docs/TODO.md` §0.1.
- `TOOL-UI-ROWS` phases (a)–(f) are untouched.
