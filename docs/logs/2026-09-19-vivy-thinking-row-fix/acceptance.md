# Acceptance — thinking row: one live renderer, left-to-right summary

Human view, on the running dev page (`http://127.0.0.1:3015`) after Vite picks
up the change:

1. **One thinking affordance.** Send a prompt that makes VIVY think. While the
   run is in flight the transcript shows exactly one thinking element per
   reasoning segment — the collapsed row `🧠 思考过程 · <summary>`. No card bubble
   with a second collapsed 思考过程（进行中） disclosure and no `…`-only bubble
   appears next to it.
2. **One answer bubble.** When the answer streams, its text appears in one
   assistant bubble. No twin bubble carries the same text.
3. **Left-to-right thinking.** The collapsed row's summary starts immediately
   after 思考过程 and grows to the right, clipped with an ellipsis at the row's
   right edge — it no longer floats at the right edge and grows leftward.
4. **Settled state unchanged.** When the run ends, the same row stays in place
   and now shows the first line of the reasoning at the same left position;
   expanding it still reveals the full reasoning text, and expanding still pins
   the header.
5. **History unaffected.** Older sessions whose run events are not loaded still
   render through the projection (user/assistant bubbles + tool-result cards),
   with no reasoning row because the projection carries none.

Machine-checkable equivalents: `ui/src/components/chat/ChatView.test.tsx` (one
reasoning row, no `<details>`, text once, projected message reused, projection
fallback) and the new case in `ui/src/components/chat/ReasoningRow.test.tsx`
(running summary uses the same left-aligned truncating element as the settled
one). Live evidence and the pre-fix reproduction are in `verification.md`.
