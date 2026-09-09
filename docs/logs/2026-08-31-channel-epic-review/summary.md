# Super Channel EPIC (C1–C7c) comprehensive review report (summary)

Date: 2026-08-31. Subject: `feat/channel-c7c` @ eb0cba0 (9 implementation commits, 151 files +16.5k/−1.3k, baseline 82ecf14).
Method: mechanical gate sweep (run directly by the coordinator) + six independent review lanes (fresh subagents, isolated from the implementation context) + fix round + gate rerun + read-only merge rehearsal.

## Verdict: mergeable (after the fix round)

All six lanes were **PASS, with zero blockers**. The fix round handled 12 items (1 code defense + 2 verify-rule reinforcements + 2 pack hygiene items + 2 plugin-lifecycle items + 3 UI presentation items + 2 documentation reconciliations); the gate rerun `just ci` exited 0. The merge rehearsal between the branch chain and `main` had **zero conflicts**.

## Mechanical gate sweep (all measured, not quoted from old records)

- `just ci` exited 0 (fmt-check / vet / test / headless-compile / ui-ci, 172 tests).
- Per-module checks for the five plugins (an existing gate blind spot): gofmt 0 dirty, vet 0, all tests green, **`-race` all green**.
- verify matrix: all 5 real plugins passed; all 8 bad-* fixtures exited 1.
- pack matrix: each of the 5 plugins produced a candidate, and every EXE linked its own SDK; the default body had **zero** telego/dingtalk/lark/botgo/discordgo/pion dependencies according to `go list -deps`; `go.mod`/`go.sum` had no byte differences from 82ecf14; `zz_register.go` still has `return nil`.
- Storage: sqlite 17.9s green + postgres (DSN-gated SKIP, CH-C1-N5 remains OPEN—no Docker/5432 locally).
- UI: typecheck/test/build green; `just ui-e2e` **6 passed, 2 failed—the failure set exactly matched baseline 82ecf14** (full runtime flow + welcome-wizard, confirmed by rerunning the baseline in a `git worktree`), an existing e2e baseline-broken issue rather than an EPIC regression; recorded as §0.1 TEST-3.
- L5 browser smoke (run directly by the coordinator, 3015): both scenarios passed—the default body empty state (no email/neuro-link, no add button) + packed Telegram candidate (card/exact failure reason/fail-closed editor wording/token_env name only)—matching the state when C5 landed.

## Six lane conclusions and representative findings

| Lane | Verdict | Representative findings (see findings.md) |
|---|---|---|
| L1 Contract compliance | PASS | 3 unboarded registration gaps (§8 error-classification slot, unimplemented verify picoclaw line, per-seam inspect not boarded); §12 payload should be written back into the contract |
| L2 Kernel + security | PASS | `deliverCompleted` nil-channel panic shape (unreachable in assembly order) → fixed; no `allow_from` bypass; zero key leakage |
| L3 Adapter cross-cutting | PASS | **DingTalk becomes deaf after a network-level silent disconnect** (SDK semantics, comment corrected + §0.1 CH-C6-N3); DingTalk/Feishu restart latches not reset → fixed; all 5 SDK claims confirmed from source |
| L4 SDK/pack | PASS | Listen ban could be bypassed through a method call → ban widened; pack silently dropped replace/exclude → changed to explicit error; no test for two-standalone-module pack → added; picoclaw import ban missing → added |
| L5 UI | PASS | inspect failure showed the wrong empty state → fixed; wizard wording overpromised → fixed; tutorial wording stale → fixed; prohibited "unlimited" wording remained → fixed |
| L6 Documentation board | PASS | ghost branch name c7a (commit actually landed on c6 line) → docs corrected; `Settings()` not written back into contract/spec → added; stale UI-CHANNELS-BE line → closed |

## Fix round (all 12 items landed, verified by the coordinator)

Code: dispatch nil-channel guard (+test), widened Listen ban (any receiver's ListenAndServe*/ListenPacket + tls.Listen, + bad-channel-listen2 fixture), picoclaw/.workspace import ban (+fixture), explicit error for non-agent-vivy replace/exclude in pack (+test), TestPackTwoStandaloneModules + deduplication of repeated --with values, reset of DingTalk/Feishu Start stopped latches (+TestStartAfterStopStartsFresh, also eliminating the potential Feishu hang), and four UI fixes for error state/wizard wording/tutorial wording/prohibited wording.
Docs: added CH-R-1/4/5 and CH-C6-N3 to TODO §0.1, fixed CH-C1-N2/N3, changed UI-CHANNELS-BE → DONE; corrected the branch wording in §0.2.7 and CH-C7a/C7b; filled the missed CN-17 in the V0 docs; added `Settings()` to CHANNEL-PACK §9.3 and PLUGIN-SPEC §4 (symbol-level alignment for the method added in C4).

## Merge rehearsal (read-only)

`git merge-tree` (merge-base HEAD↔main) reported **0** conflict blocks. Merge checklist:

- The branch chain is clean (`git status` clean at eb0cba0); 25 unpushed commits (including the 16 that led with the contract branch).
- Two paths: **A. Merge the entire `feat/channel-c7c` chain into main**—retains the 9 slice commits + contract documentation commit and the most complete history; recommended. B. squash—one commit, loses slice granularity, and makes rollback to a single slice difficult. B is not recommended.
- The main root tree currently has dirty areas unrelated to this EPIC (service_test.go, studio, failure.ts, etc.); before merging, their owning lanes must handle or stash them so they are not mixed in.

## Explicitly not done (declaration of non-work)

- Real vendor smoke tests ×5 (no credentials; each plugin's acceptance includes a manual script); the real Postgres path (no Docker, CH-C1-N5 open); C8/C9 (requires explicit request/proposal).
- Fixes for the two e2e baseline-broken cases (outside this EPIC's scope; recorded as §0.1 TEST-3).
- Note-level findings (about 30) were not fixed one by one; see findings.md for the complete set.
