# Block 4 — composer split and alignment

## What changed

`ui/src/components/chat/ChatInput.tsx` (530 lines, five responsibilities in one
file) is deleted and replaced by `ui/src/components/chat/composer/`:

| file | owns |
| --- | --- |
| `composer-root.tsx` | the `ChatInput` export; draft value, pending attachments, notice, send lock, thinking mode, attachment gate, key/paste/blur handling, and the column layout |
| `composer-toolbar.tsx` | the top control row (history, attach, thinking, permission, approval trigger); wraps instead of scrolling |
| `composer-thinking-menu.tsx` | the nine thinking modes on one Radix menu |
| `composer-permission-menu.tsx` | cautious/smart/trusted switch plus the Trusted confirmation dialog |
| `composer-queue.tsx` | kernel steer/follow-up pills and the local FIFO; two-line clamp with tooltip, remove per pill, clear all |
| `composer-approval.tsx` | the quick-approval trigger badge and its floating card (plan named this `approval-quick-card.tsx`; it hosts the existing `ApprovalQuickCard`, so the file is named for the composer slot) |
| `composer-textarea.tsx` | the single autogrow owner |
| `composer-footer.tsx` | context ring, workspace chip, notice line and the send/steer/follow-up/send-now/cancel cluster (extracted from the root; the root is 288 lines of state and layout instead of 530) |

`HistoryPicker` moved out of the composer into `ChatView.tsx`, which now owns
`pickerOpen` and passes `onOpenHistory` down; the composer no longer renders a
dialog it does not control.

### Defects fixed

- **Double autogrow cap.** The old textarea capped at `max-h-40` while
  `ui/textarea.tsx` forced `min-h-[60px]`, and a second effect re-measured the
  same node. `composer-textarea.tsx` is now the only owner: `min-height:
  var(--control-lg)`, `max-height: 40vh`, internal scroll past the cap.
- **Column misalignment.** Transcript and composer both use `max-w-content`
  (52rem); the composer was `max-w-3xl`.
- **Toolbar horizontal scroll.** `overflow-x-auto` became `flex-wrap`; at 390
  the controls measure 364/364, so no overflow menu was needed (the plan
  allowed "wrap plus an overflow menu"; wrap alone measures clean).
- **Queue pill clipping.** `truncate` + `max-w-40` + `overflow-x-auto` became
  `line-clamp-2` + `break-words` + `title`, in a wrapping row.
- **Workspace chip squeeze at 390.** `WorkspaceSelector` lost its stock
  `text-xs`/`hover:bg-muted` pair for `text-meta`/`hover:bg-hover`; the footer
  wraps instead of crushing the chip to 30px.
- **Approval badge overhang.** The pending-count badge sat 2px outside its
  button box (`-right-0.5`), which any future `overflow-hidden` ancestor would
  clip; it is now inset at `right-0`.

### Token conversions

`ComposerCommands.tsx`, `ContextReferenceChip.tsx`, `WorkspaceSelector.tsx`
moved to semantic tokens and the type scale. Ratchet baselines shrink:
`src` palette 154 → 148, `src` text-size 55 → 52 (ChatInput.tsx and
ContextReferenceChip.tsx entries deleted).

### i18n

No new keys. Five Web-only keys that Blocks 3–4 use but the cross-face
contract never classified (`chat.code`, `chat.copyCode`, `chatInput.sendNow`,
`deliverableCard.showLess`, `deliverableCard.showMore`) are now in
`scripts/i18n-cross-face-contract.json`, so `check-i18n-cross-face.js` exits 0;
`docs/TODO.md` I18N-CHECK-RED is narrowed to the three `failure.ts` Han
markers.

## What was explicitly not done

- No overflow menu for the toolbar (wrap measures clean at 390).
- Sidebar session-row and header model truncation remain; they are Block 5.
- `just ci` Go lanes not run on this host (see verification).
