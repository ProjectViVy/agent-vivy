# Acceptance — Eino core upgrade

1. `go version -m` on a built `vivy` shows `github.com/cloudwego/eino v0.9.21`.
2. Upgrade with a paused run: a run waiting on an approval, question or Plan
   review before the upgrade shows, after the new binary starts, as **failed**
   with "Vivy was updated while this run was waiting, and a paused run cannot
   continue on a different engine version. Its pending request was cancelled.
   Please start the task again." Its card no longer appears as actionable in
   Review Center (approval/question status `cancelled`, Plan review
   `cancelled`). Direct rollback to the historical old binary with pending work is unsupported. Before rollback, complete cleanup under the current fixed binary and verify no active/pending work or cleanup intents remain. See the corrected policy and `../2026-10-18-pr80-recovery/verification.md` for actual-binary evidence.
3. Runs that were not paused, and new runs, behave as before.
4. `go test -race ./internal/runtime -run 'TestNudge|TestToolFailure'` passes
   (it reported the ToolsNode race before).
5. `docs/DEFER.MD` no longer lists `EINO-CORE-UPGRADE` or
   `EINO-TOOLSNODE-ERR-RACE`; both are in `docs/COMPLETE.MD` as DONE.
