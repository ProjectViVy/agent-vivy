# Acceptance: PR #79 review fixes

A human can tell this worked by checking:

1. **Default build, Memory page.** Open `http://127.0.0.1:3015/memory` on the split pair (`just dev`). The previous memory view shows; there are no Recap tabs and no raw `recap_scope_denied` text.
2. **Recap still works where it is wired.** `ui/playwright.issue58.config.ts` (resolver present) opens the Recap tabs and passes.
3. **A bad provider stream does not brick a session.** Stream tool arguments containing byte `0xff` in a coding session: that run fails, and the session history still loads and accepts a new run.
4. **A cancelled tool call does not stop Recap.** In a Recap session, cancel during a tool call, then send another message. The next request is built; the cancelled run appears as an excluded interaction (no transcript in the Recap input).
5. **A mistyped tool argument is recoverable.** In a Recap session the model gets the argument error as a tool result and can retry; the run does not fail.
6. **Composer.** Pick `max`, switch to a model that does not list it: the trigger reads "Follow default" and no level is sent.
7. **Approvals.** With a pending request from another session, the quick card says "From another session: <title>".
8. **Laputa patch.** In Laputa, `go test ./actmem` passes, including the cancel-before-replacement test.
