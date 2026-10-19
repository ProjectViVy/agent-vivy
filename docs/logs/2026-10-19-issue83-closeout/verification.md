# Verification evidence

## Candidate

- Latest-main integration base: `e96092966cfbfa443648374948e7e7e16dd01683`.
- Final candidate SHA and executable hashes: pending final commit and clean standalone build.
- Laputa lock target: `0e0027c5f60ad67614d3613c2d5e21d567c4b412`; Laputa PR #11 remains open.

## Local Windows verification

- `just ci` — PASS, exit 0, Go 1.26.4. This ran on the integrated candidate worktree and covered formatting, UI typecheck/tests/build, `go vet ./...`, ordinary Go tests, the full DIVA Memory Loop overlay, six `diva-recall` acceptance cases, all 22 plugin modules, and BML tests. The formal DIVA overlay also passed separately with `node scripts/memory-loop.mjs` (`command-line-arguments` 985.134s).
- `just race-ci` — PASS, exit 0, native Windows with race-enabled SQLite. `internal/app`, `internal/rpc`, `internal/runtime`, and `internal/storage/sqlite` race suites completed without race reports; the recovery cleanup retry, resumable Approval, cancellation, Plan, and Question recovery tests were included.
- `node --check scripts/pending-run-binary-smoke.mjs` and `node --check scripts/build-eino0913-rollback.mjs` — PASS.
- `node scripts/build-eino0913-rollback.mjs --output <scratch>/vivy-eino0913.exe --go 'C:/Program Files/Go/bin/go.exe'` — PASS. The builder uses a scratch `-modfile`, leaves project `go.mod` unchanged, and verifies the built binary embeds Eino v0.9.13.

## Actual binary and isolated SQLite acceptance

All cases launch native Windows executables against separate scratch databases, seed paused Approval/Question/Plan runs through RPC, kill the owning process without graceful cleanup, wait for the real 30-second lease, and inspect durable rows/events/checkpoint pointers after restart. The fake model runs only while seeding; recovery must not increase the request count.

- Historical baseline: standalone clean build of `e96092966cfbfa443648374948e7e7e16dd01683`, Eino v0.9.13, Laputa `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`, SHA-256 `EF27BD29102B12D31F9D5392A71DC78FD9D2F33BD521DF27CDE2728E1093AF75`; build info reports `vcs.modified=false`.
- Historical matrix command: `node scripts/pending-run-binary-smoke.mjs --old <historical.exe> --new <candidate.exe> --output <fresh-scratch-dir> --port 18830 --go 'C:/Program Files/Go/bin/go.exe'`.
- Historical matrix result: upgrade is PASS; direct rollback and cleanup-before-rollback are both expected fail-closed cases. The old binary rejects migration 44 with `unknown migration version 44` before recovery, both when requests are pending and after candidate cleanup. The database state remains unchanged by the rejected start. This demonstrates that cleanup alone does not make the unpatched historical binary a supported target.
- Compatibility build command: `node scripts/build-eino0913-rollback.mjs --output <fresh-scratch-dir>/vivy-eino0913.exe --go 'C:/Program Files/Go/bin/go.exe'`.
- Compatibility matrix command: `node scripts/pending-run-binary-smoke.mjs --old <compatibility.exe> --new <candidate.exe> --output <fresh-scratch-dir> --port 18860 --go 'C:/Program Files/Go/bin/go.exe' --compat-rollback`.
- Compatibility matrix result: PASS for upgrade, direct compatibility rollback, and cleanup-before-rollback. On direct rollback, all three pending runs become failed once for the exact Eino-version mismatch; Approval and Question rows/events and the Plan review/event are cancelled once. Checkpoint IDs/generations/checksums remain unchanged, no recovery model requests occur, and repeated restarts add or lose no events. The cleaned rollback case also remains stable.
- Detailed machine-readable summary: `binary-results.json` in this directory. The exact final-source SHA, candidate and compatibility executable hashes, and final report paths are populated after the clean standalone build.

## Remote checks

- GitHub checks on prior PR heads are not evidence for this integrated candidate. The exact final SHA is not yet published; its GitHub checks will be recorded separately after the PR update.
- PR #84 run `39166916420` passed at its earlier SHA `47d9c16a584026af40d5c1884ab946dc6695c5dc`; [run](https://github.com/ProjectViVy/agent-vivy/actions/runs/39166916420).
- PR #85 run `39168489073` passed at its earlier SHA `ba9642bd0cae7005359da94ebbe994969d224fe5`; [run](https://github.com/ProjectViVy/agent-vivy/actions/runs/39168489073).

## Remaining open work

- Exact-SHA GitHub CI remains pending until the candidate is pushed and the PR is updated.
- Laputa PR #11 remains open; its Windows Mentle blocker is not waived.
- Full AgenticModel migration and Provider milestones B5/B6/B7 remain open. No complete Provider migration is claimed.
