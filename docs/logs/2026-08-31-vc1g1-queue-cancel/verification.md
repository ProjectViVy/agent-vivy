# verification — VC-1g-1

All commands ran in worktree `agent-vivy-vc0` (branch `feat/vc1a-bash-tool`).

| Command | Result |
| --- | --- |
| `pnpm typecheck` (ui/) | exit 0 |
| `pnpm test` (ui/) | 24 files / **194 passed** (including 4 new queue tests: queue while busy, dispatch after completion, retain after failure, clear on session switch) |
| `just ci` | **exit 0** (0 FAIL lines in the log; 26 Go packages + UI typecheck/test/build all green) |

New test points (`ui/src/lib/store.test.ts`):

1. While a run is active, `startRun` does not call `api.startTurn`; the message enters `queuedMessages` (text/mode are correct).
2. After the `run.completed` event (terminal refresh complete), the first queued message is dispatched automatically: `api.startTurn('s1', 'second message', 'normal', undefined)`; the queue is cleared.
3. After a `run.failed` event, the queue is **retained** and not dispatched; `runError` displays normally.
4. The queue is cleared after `selectSession` switches sessions.

Smoke note: a complete UI walkthrough of queuing/two-stage cancellation requires an active run, and an active run requires
a real provider key (after TEST-1, this repository has no mock provider). The component and store behavior is covered by the unit tests
and typecheck above; following the same smoke-policy exception recorded in `docs/logs/2026-08-31-vc1f-ui-diff/verification.md`,
manual walkthroughs are reserved for a runtime with a key. The verification path is "send while running → queue pill appears →
automatic dispatch after completion; two-stage Esc/Stop."
