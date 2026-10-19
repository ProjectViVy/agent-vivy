# Verification evidence

## Candidate

- Latest-main integration base: `e96092966cfbfa443648374948e7e7e16dd01683`.
- `git ls-remote origin refs/heads/main` on 2026-10-19 — PASS; remote `main` still points to `e96092966cfbfa443648374948e7e7e16dd01683`.
- Tested implementation source SHA: `ac461b48dc5677767fda8522cbce1e64efc2c4f8` (includes current PR #84 head `47d9c16a`). The later evidence-only documentation commit does not change executable source.
- Laputa lock target: `0e0027c5f60ad67614d3613c2d5e21d567c4b412`; Laputa PR #11 remains open.

## Local Windows verification

- `just ci` on implementation SHA `ac461b48dc5677767fda8522cbce1e64efc2c4f8` — PASS, exit 0, Go 1.26.4. This final-source run passed formatting, UI checks/build, `go vet ./...`, `go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...`, the DIVA overlay (`command-line-arguments` 968.164s), all six `diva-recall` acceptance cases, all 22 plugin modules, and BML (`go test -p=1 -count=1 -timeout=5m ./...`, 8.703s). The run log is `.workspace/pr80-binary/just-ci-ac461-final.log` (ignored local scratch).
- `just race-ci` — PASS, exit 0, native Windows with race-enabled SQLite. `internal/app`, `internal/rpc`, `internal/runtime`, and `internal/storage/sqlite` race suites completed without race reports; the recovery cleanup retry, resumable Approval, cancellation, Plan, and Question recovery tests were included. This full run preceded the merge of PR #84's test-only Windows timeout adjustment; its race-build predicate remains true on Windows. The focused recovery race run below uses the final implementation SHA.
- `go test -count=1 -timeout 10m ./internal/runtime -run '^(TestRecoverCleanupIntentWriteFailures|TestRecoverCleanupRetriesInteractionWrites|TestRecoverCleanupRetriesPlanAfterTerminal|TestRecoverAcrossEngineVersionCancelsPendingApproval|TestRecoverAcrossEngineVersionCancelsPendingQuestion|TestRecoverAcrossEngineVersionCancelsPendingPlanReview|TestServiceRecoverResumableApproval)$'` — PASS on final implementation SHA `ac461b48dc5677767fda8522cbce1e64efc2c4f8` (`ok agent-vivy/internal/runtime 37.981s`).
- `go test -race -count=1 -timeout 15m ./internal/runtime -run '^(TestRecoverCleanupIntentWriteFailures|TestRecoverCleanupRetriesInteractionWrites|TestRecoverCleanupRetriesPlanAfterTerminal|TestRecoverAcrossEngineVersionCancelsPendingApproval|TestRecoverAcrossEngineVersionCancelsPendingQuestion|TestRecoverAcrossEngineVersionCancelsPendingPlanReview|TestServiceRecoverResumableApproval|TestServiceCancelDuringApprovalPublishClosesDurableApproval)$'` — PASS on clean standalone source SHA `ac461b48dc5677767fda8522cbce1e64efc2c4f8` (`ok agent-vivy/internal/runtime 91.416s`).
- `node --check scripts/pending-run-binary-smoke.mjs` and `node --check scripts/build-eino0913-rollback.mjs` — PASS.
- `node scripts/build-eino0913-rollback.mjs --output .workspace/pr80-binary/vivy-compat-rollback-ac461-eino0913.exe --go 'C:/Program Files/Go/bin/go.exe'` — PASS. The builder uses a scratch `-modfile`, leaves project `go.mod` unchanged, and verifies the built binary embeds Eino v0.9.13.
- `node scripts/memory-loop.mjs` on implementation SHA `ac461b48dc5677767fda8522cbce1e64efc2c4f8` — PASS, exit 0; `command-line-arguments` completed in 926.531s. An earlier post-merge invocation overlapped other verification jobs and exited nonzero, but its terminal output did not preserve the failing test diagnostic. A serial rerun passed, and the final `just ci` run separately passed this overlay in 968.164s.

## Actual binary and isolated SQLite acceptance

All cases launch native Windows executables against separate scratch databases, seed paused Approval/Question/Plan runs through RPC, kill the owning process without graceful cleanup, wait for the real 30-second lease, and inspect durable rows/events/checkpoint pointers after restart. The fake model runs only while seeding; recovery must not increase the request count.

- Historical baseline: standalone clean build of `e96092966cfbfa443648374948e7e7e16dd01683`, Eino v0.9.13, Laputa `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`, SHA-256 `EF27BD29102B12D31F9D5392A71DC78FD9D2F33BD521DF27CDE2728E1093AF75`; build info reports `vcs.modified=false`.
- Candidate: clean standalone build from `ac461b48dc5677767fda8522cbce1e64efc2c4f8`, Eino v0.9.21, SHA-256 `e2240d3698104643c36e90eeba4d7d61392c6bf0969b3b5bab54b2af1a24f4bb`; `vcs.modified=false`.
- Compatibility rollback: build from the same clean source SHA with `scripts/build-eino0913-rollback.mjs`, Eino v0.9.13, SHA-256 `0143769b753bc38c97b09692769ad8c4aeb164a767181adf04b56ee48e206443`; `vcs.modified=false`.
- Historical matrix command: `node scripts/pending-run-binary-smoke.mjs --old C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy\.worktrees\issue83-final\agent-vivy\.workspace\pr80-binary\old-e960-standalone\vivy-old-e960-eino0913.exe --new .workspace/pr80-binary/vivy-candidate-ac461-eino0921.exe --output .workspace/pr80-binary/final-historical-e960-ac461 --port 18880 --go 'C:/Program Files/Go/bin/go.exe'`.
- Historical matrix result: upgrade is PASS; direct rollback and cleanup-before-rollback are both expected fail-closed cases. The old binary rejects migration 44 with `unknown migration version 44` before recovery, both when requests are pending and after candidate cleanup. The database state remains unchanged by the rejected start. This demonstrates that cleanup alone does not make the unpatched historical binary a supported target.
- Compatibility matrix command: `node scripts/pending-run-binary-smoke.mjs --old .workspace/pr80-binary/vivy-compat-rollback-ac461-eino0913.exe --new .workspace/pr80-binary/vivy-candidate-ac461-eino0921.exe --output .workspace/pr80-binary/final-compatible-eino0913-ac461 --port 18890 --go 'C:/Program Files/Go/bin/go.exe' --compat-rollback`.
- Compatibility matrix result: PASS for upgrade, direct compatibility rollback, and cleanup-before-rollback. On direct rollback, all three pending runs become failed once for the exact Eino-version mismatch; Approval and Question rows/events and the Plan review/event are cancelled once. Checkpoint IDs/generations/checksums remain unchanged, no recovery model requests occur, and repeated restarts add or lose no events. The cleaned rollback case also remains stable.
- Detailed machine-readable summary: `binary-results.json` in this directory. Full per-process reports are under the ignored `.workspace/pr80-binary/` scratch path in the standalone acceptance clone.

## Remote checks

- GitHub checks on prior PR heads are not evidence for this integrated candidate. The exact final SHA is not yet published; its GitHub checks will be recorded separately after the PR update.
- PR #84 run `39166916420` passed at its earlier SHA `47d9c16a584026af40d5c1884ab946dc6695c5dc`; [run](https://github.com/ProjectViVy/agent-vivy/actions/runs/39166916420).
- PR #85 run `39168489073` passed at its earlier SHA `ba9642bd0cae7005359da94ebbe994969d224fe5`; [run](https://github.com/ProjectViVy/agent-vivy/actions/runs/39168489073).

## Remaining open work

- Exact-SHA GitHub CI remains pending until the candidate is pushed and the PR is updated.
- Laputa PR #11 remains open; its Windows Mentle blocker is not waived.
- Full AgenticModel migration and Provider milestones B5/B6/B7 remain open. No complete Provider migration is claimed.
