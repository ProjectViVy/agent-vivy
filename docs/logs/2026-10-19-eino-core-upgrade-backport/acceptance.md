# Acceptance

Date: 2026-10-19

A human can confirm this delivery without reading the diff:

1. **Pin.** `go list -m github.com/cloudwego/eino` prints
   `github.com/cloudwego/eino v0.9.21`. `go list -m -f '{{.Path}} {{if .Replace}}=> {{.Replace.Path}}{{end}}' github.com/cloudwego/eino-ext/components/model/claude`
   prints the `third_party/eino-claude` replacement.
2. **Upgrade across a paused run.** Start a run that suspends on an approval
   (e.g. the approval fixture used by `TestRecoverAcrossEngineVersionCancelsPendingApproval`),
   stop the process, and restart it with a binary built against a different Eino
   version. Recovery leaves exactly one terminal event, the run fails with the
   "Vivy was updated while this run was waiting…" text, and the approval row is
   `cancelled` before that terminal. The Review Center shows no actionable card
   for the run afterwards; the user starts the task again.
3. **Lost checkpoint at the same version.** Delete the stored checkpoint and
   restart: the run still fails, keeps the generic restart message, and its
   pending approval is cancelled. `TestRecoverWithMissingCheckpointCancelsPendingApproval`
   is the automated form.
4. **Plan review.** The same upgrade closes a pending Plan review as
   `cancelled` after the terminal instead of leaving it open
   (`TestRecoverAcrossEngineVersionCancelsPendingPlanReview`).
5. **No conversation.** `go test -run Recover ./internal/runtime/` passes, so
   the pre-existing recovery paths are unchanged.
6. **Claude stream.** The provider suite passes against the vendored component;
   no public API, protocol or callback behavior changed, per
   `third_party/eino-claude/PATCHES.md`.

What a human cannot confirm yet: a real-provider Claude race-clean run and the
full `just ci` candidate gate; both are PR/final-candidate steps, not claimed
here.
