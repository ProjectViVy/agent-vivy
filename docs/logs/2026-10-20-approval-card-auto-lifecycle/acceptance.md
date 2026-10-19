# Acceptance — approval card appears on arrival and leaves after a decision

1. Start the split pair with `just dev` and open `http://127.0.0.1:3015`.
2. Trigger a tool that needs approval (for example, ask the agent to write a
   file under the default smart preset). When the review arrives, the quick
   approval card opens above the composer without clicking the shield, and the
   shield badge shows the pending count.
3. Click **Approve** (or **Deny**). The buttons show their busy state while the
   decision is in flight; only after the response and the review refresh succeed
   does the card disappear.
4. If the decision fails (for example, a stale review), the card stays open,
   keeps the same review visible, and shows the error; retry or dismiss it.
5. Close the card manually while reviews are still pending. It stays closed
   until another pending review arrives; it does not immediately pop back.
6. With no pending reviews, no card is shown. The context-meter compact control
   and the rest of the composer continue to work.
