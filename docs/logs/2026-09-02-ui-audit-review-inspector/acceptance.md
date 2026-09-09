# Acceptance

How a human confirms it:

1. Open http://127.0.0.1:3015 → Settings → Vivy features → the Run Inspector card has a
   fourth "Reviews (0)" tab; with no reviews it shows "This run has no reviews or questions."
2. Start a conversation that triggers tool approval (for example, a dangerous bash command).
   Run Inspector → Reviews tab shows an approval card exactly matching Review Center:
   audit fields, risk warning, redacted parameters, and Approve/Reject buttons. Approve it in
   the inspector, and the same request settles in the approval-center queue.
3. Review Center page behavior is unchanged: details are still the same card (now from the
   shared component), and list/refresh/rewind are unaffected.
