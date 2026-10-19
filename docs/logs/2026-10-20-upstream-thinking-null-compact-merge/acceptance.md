# Acceptance — thinking settings no longer take down the frame

1. Start the split pair with `just dev` and open `http://127.0.0.1:3015`.
2. Open **Settings**. The page and sidebar stay mounted; the **Thinking level**
   card renders (or shows its unsupported state) instead of a full-frame
   `Vivy UI Module failed` diagnostic. This is the bug that was reported with a
   DeepSeek model that declares no thinking levels.
3. Return to the chat composer. When the active session reports
   `compaction_enabled`, the context-usage ring and percentage are now one
   button: clicking it opens the **Compact context** popover with the
   instructions field and **Compact now** action. Escape closes it.
4. When compaction is unavailable (disabled or no active session), the meter
   remains a passive progressbar and no dead compact button appears.

Expected: no `Cannot read properties of null (reading 'length')` console error,
no root error boundary, and the existing compact request still goes through
`session/compact`.
