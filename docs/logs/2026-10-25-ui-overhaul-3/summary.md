# Block 3 — chat transcript surfaces

## What changed

- `components/chat/MessageBubble.tsx` is now a dispatcher. The three surfaces it
  used to hold live in their own files:
  - `message-user.tsx` — user bubble, edit mode, attachments, hover actions.
  - `message-assistant.tsx` — assistant bubble, provenance line, copy /
    regenerate / rewind / fork, rewind and fork confirm dialogs.
  - `tool-result.tsx` — tool-result card (diff, path, raw result disclosure).
- `markdown-block.tsx` — one markdown renderer for chat. Fenced code becomes a
  titled block with a copy button; streaming shows a tokenised caret instead of
  the old `…` marker.
- `message-actions.tsx` — shared action-button geometry and timestamp format.
- `lib/clipboard.ts` — clipboard write with an `execCommand` fallback for
  restricted webviews (moved out of `MessageBubble.tsx`).
- `ToolRow.tsx` / `ReasoningRow.tsx`: fixed `h-6` became `min-h-7` with a
  two-line clamped summary; expanded bodies use the semantic type sizes
  (`text-2xs`, `text-meta`, `text-ui`, `text-read`); failure tone uses the
  `danger` tokens.
- `DeliverableCard.tsx`: `line-clamp-2` + `max-h-40` replaced by an explicit
  12-line preview with Show all lines / Collapse preview; paths break instead of
  truncating.
- `ChatView.tsx`: transcript column uses `--content-max`; loading state is a
  three-block skeleton.
- `styles.css`: `vivy-caret-blink` + `.streaming-caret` rules (reduced-motion
  respected).
- `components/ui/scroll-area.tsx`: the Radix viewport's inline-styled
  `display: table` child is re-declared as a full-width block. Without this the
  transcript scroller grew to the widest min-content child (822px at a 390px
  viewport) and clipped every wrap instead of wrapping. This was the root cause
  of the narrow-viewport clipping, not a per-bubble defect.
- i18n: `chat.code`, `chat.copyCode`, `deliverableCard.showMore`,
  `deliverableCard.showLess` (en + zh).

## Explicitly not done

- Composer-side surfaces (approval quick card, queue pills, toolbar) are Block 4.
  The one remaining truncated element in the 390px approval screenshot is the
  composer's workspace chip, owned by that block.
- No change to run/queue/store behaviour; `ChatView` handlers and memos are
  untouched.
- Plugin UI modules still carry their own stock palette classes (Block 7).
- Real-provider streaming was not exercised: the split pair ran against a local
  OpenAI-compatible mock through the frozen-env provider path.
