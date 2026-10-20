# Acceptance

D03 is accepted for the exact private operation/approval binding and durable
outcome boundary. The index remains the sole Story status board.

A user can approve one specific tool call, deny it without a write, or restart
the backend while approval is pending and decide the recovered call. The
result reaches the next model request, and the Run completes once with the
composer available again. Sibling tools do not inherit another call's approval
or replay an already completed effect.

The recorded split-browser checks used only a disposable home and local
streaming mock. [Approval/denial](browser-smoke.png),
[restart recovery](browser-restart.png), and
[redacted approval preview](browser-privacy.png) show the visible outcomes.
The [verification record](verification.md) ties these views to exact Run/call
identities, actual files, persisted classifications and terminal counts.

Storage fixtures show the failure behavior a user cannot safely infer from
a success screen: a lost claim remains fenced; an old unbound operation cannot
execute; different retry bytes conflict; different completion classifications
conflict; a Journal failure rolls completion back. Baseline conversion copies
retain historical records and never reconstruct authority.

To reproduce the product checks:

1. Start a source-built split backend and Vite at :3015 with a new disposable
   home and workspace, then initialize Persona through its normal owner.
2. Route an embedded provider to a local streaming mock that requests one
   write_file call, then returns a final response after its tool result.
3. Approve one call and deny another. Inspect each Run workspace for the
   expected exact nonce or absence.
4. Suspend a third call at approval; SIGKILL only that backend, let its lease
   expire, restart the same home, and approve the recovered call in the UI.
5. Compare each immediate next-model tool message and call ID with its
   persisted outcome; confirm one terminal and no active/pending work.
6. Use fake secret canaries to verify private request/provider bytes survive
   while public events, review RPCs, previews and logs remain redacted.

Full conversion publication still requires D08/D12/D14. Native Windows
full-product and packaging gates are not claimed by these WSL scoped results.
Rollback uses the matching executable and a quiescent backup/copy; never point
an old executable at a converted store or overwrite newer data.
