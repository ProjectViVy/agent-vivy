# Acceptance — quick approval card above the composer

1. Start the split pair with `just dev` and open `http://127.0.0.1:3015`.
2. Open a chat session whose backend has a pending approval or question. The
   composer shield shows the badge with the pending count.
3. Click the shield. A compact card appears above the input row, right aligned,
   naming the gated command (or question), the remaining count and the first
   risk note. It offers **Approve** / **Deny** for approvals and a link to the
   approval center; clicking the shield again or the close button hides it.
4. Decide from the card. The badge count, the in-chat review sheet, the review
   center and `/approvals` all reflect the same decision.
5. With nothing pending, clicking the shield shows the empty card
   (`当前没有待处理事项` / `No pending items`) instead of an empty popover.

The context-meter compact control and the "thinking level" settings card remain
usable in the same composer/frame.
