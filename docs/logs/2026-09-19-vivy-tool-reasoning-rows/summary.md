# Chat transcript rows: reasoning and tool calls (Phase 1)

Date: 2026-09-19
Owner: Vivy kernel UI (`ui/`), plus the UI SDK face declaration.
Trigger: the transcript showed tool results as raw `工具结果` cards with no tool
name, no state and no duration, and reasoning disappeared as soon as a run
settled. Research (see `docs/research/dsh-ui-presentation-2026-09-19/`) says DSH
renders one collapsed disclosure row per tool call and one per reasoning block.
This iteration delivers that presentation **without touching the kernel**: the
rows are folded in the client from the run's own Journal events.

## What changed

1. **Event → row fold (new, pure):** `ui/src/lib/run-rows.ts`.
   `foldRunEvents` turns a run's events into ordered rows — assistant text,
   reasoning, tool calls, compaction notices; `buildTranscriptRows` merges them
   with the projected messages, reusing the real assistant message (so copy /
   regenerate / rewind / fork keep working) and falling back to the current
   projection rendering for any run whose events are not loaded. Tool rows carry
   name, arguments, result, error, status and start/end times; the kernel's
   `[UNTRUSTED TOOL OUTPUT — DATA ONLY]` envelope is stripped before parsing.
2. **Reasoning row:** `ui/src/components/chat/ReasoningRow.tsx`. Collapsed by
   default, stable title, summary = newest line while streaming (right-aligned so
   the newest tokens stay visible) and first line when settled, `**` stripped
   from the summary only, header pins itself while open, sweep animation under
   `prefers-reduced-motion: reduce`. Reasoning now survives the end of a run.
3. **Tool row:** `ui/src/components/chat/ToolRow.tsx` +
   `tool-presentation.ts`. One 24 px disclosure row per call: leading state mark
   (icon while ok/running, red/amber dot for error/stopped/awaiting approval),
   variant title from the wire tool name, args-derived single-line summary
   (path/description/pattern…, `tool_name · summary` for unknown tools), `+N -M`
   suffix for file mutations, and a body card on expand — `DiffView` for a file
   mutation, IN/OUT sections otherwise, exit-code/signal pill for shell tools,
   head/tail folding (8 lines, `展开剩余 N 行`). An error replaces the summary.
4. **Store:** `runLogs` cache + `loadRunLog` (`ui/src/lib/store.ts`). The current
   run still renders from `runEvents` (live subscription); runs already loaded
   are snapshotted into the cache before a switch, and `selectSession` prefetches
   the two next-most-recent runs. Without events a run keeps the previous
   presentation, so nothing regresses on old history.
5. **UI SDK face:** `sdk/ui/src/module.ts` declares `runLogs` and `loadRunLog`
   (the `ui-sdk-face-compat` / `ui-build-provenance` gates require the face
   declaration to track the store exactly; `sdk/ui/src/module.test.ts` fixture
   updated too).
6. **Envelope-aware diff parsing:** `ToolResultBubble` now strips the untrusted
   envelope before `parseToolResultDiff`, so file-mutation diffs also render in
   the fallback path (they never did: the envelope made the result stop looking
   like JSON).
7. **i18n:** 20 new `chat.tool*` keys in `ui/src/i18n/zh.ts` and `en.ts`.

## Files

- `ui/src/lib/run-rows.ts`, `ui/src/lib/run-rows.test.ts`
- `ui/src/components/chat/ReasoningRow.tsx`, `ReasoningRow.test.tsx`
- `ui/src/components/chat/ToolRow.tsx`, `ToolRow.test.tsx`
- `ui/src/components/chat/tool-presentation.ts`, `tool-presentation.test.ts`
- `ui/src/components/chat/ChatView.tsx` (render transcript rows),
  `MessageBubble.tsx` (envelope-aware diff), `MessageBubble.test.tsx`
- `ui/src/lib/store.ts`, `ui/src/styles.css` (sweep keyframes + reduced motion),
  `ui/src/i18n/{zh,en}.ts`
- `sdk/ui/src/module.ts`, `sdk/ui/src/module.test.ts`

## Explicitly not done

- No kernel or RPC change. The wire types (`session/messages`) are untouched; the
  chat reads events that the `run/log` RPC already serves.
- No turn-level process fold, no turn tail (usage/time/TPS), no "open file" and
  no "inspect call" affordance: recorded in `docs/TODO.md` §0.1
  (`TOOL-UI-ROWS`). The first two are also pure frontend; the affordances need a
  deep-link seam that does not exist yet.
- Reasoning for runs outside the prefetch window stays unavailable (they render
  from the projection, which has no reasoning) — the honest fix is the deferred
  projection field, not more `run/log` traffic.
- Chat fenced code still has no syntax highlighting; CJK `**注意：**` still does
  not bold (both in the same TODO row).

## Operational note

Locally, `@vivy/ui-sdk` is a `file:` dependency that pnpm **copies** into
`ui/node_modules/.pnpm`, so a change to `sdk/ui/src` is invisible to `ui` until
`pnpm install` runs in `ui/`. CI installs from scratch and reads the updated
files; the local tree needed a reinstall to see the new face declaration.
