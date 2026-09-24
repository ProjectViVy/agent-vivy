# Verification

- TDD RED: the new wake/shutdown, failed-pause, and pending-Question restart scenarios failed for their intended missing behavior before production edits. The existing child recovery used `child.failed`; after correcting the test expectation, its worker-lost baseline passed.
- Focused GREEN: all four new scenarios passed.
- Race: `go test -race ./internal/runtime -run 'Test(Goal|WorkAdmission)' -count=1` passed, exit 0 (`internal/runtime` 9.170s), with no race report.
- Durable checks: tests read SQLite Work versions/phases and events, pending Question and run rows, child Journal cause, and Goal round/run counts after recovered answer.
- Source digest: `go run ./sdk/internal/cmd/source-hash internal ''` produced `2ebe5975afb68dae429f8777383606413f58c4560bad7afa41efe2ec2e486e3c`, refreshed in five SDK conformance entries.
- First stable-tree `just ci`: exit 1. Formatting, UI 49 files/400 tests, build, i18n, vet, app/RPC and SDK packages passed. `internal/runtime` failed only `TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries`, whose 5s terminal wait elapsed under the full Go suite (8.42s test duration). This ordinary human/Plan test does not use the changed GoalRound path.
- Unchanged-tree isolated rerun: `go test ./internal/runtime -run '^TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries$' -count=3 -v` passed 3/3 (0.98s, 2.34s, 1.32s).
- The single unchanged-tree `just ci` retry passed, exit 0: formatting, UI typecheck, 49 files/400 tests, Vite build/i18n, Go vet and all Go packages (including `internal/runtime`, `sdk/internal` 417.484s, and `sdk/internal/conformance` 112.404s), headless compile, and plugin/face vet/tests. The retry was observed in exec session `11151`; no persistent log file was captured. No code or timeout was changed between gates.
- Staged scope review: `git diff --cached --check` passed; only the two pre-existing generated UI files remain unstaged.
- Docker-required validation, including live PostgreSQL acceptance, was explicitly deferred by the user. No Docker action was taken and PostgreSQL acceptance is not claimed. The PG-3 browser/model walkthrough remains outside this Task 3 scope.

## Review fix round 1

- RED: `go test ./internal/runtime ./internal/rpc ./internal/app -run 'Test(GoalRecoveryHumanQuestionDoesNotStartGoalRound|GoalEditResponseReflectsRearmedOwnedRun|AppShutdownStopsGoalAdmissionBeforeChannelDrain)$' -count=1` failed as intended in all three packages. The human Question answer admitted one unwanted Goal round; the edit response was stale; both `App.Close` and `App.Run` admitted a run while channel `Stop` was held. After tightening test teardown, the same three failures reproduced without closed-database cleanup noise.
- GREEN: the same focused command passed (`runtime` 2.822s, `rpc` 1.438s, `app` 2.880s). The tests inspect persisted Question status, Work phase/round count, run rows, and the RPC response; the app test uses a fake channel `Stop` barrier with the Journal still open.
- Focused race: `go test -race ./internal/runtime ./internal/rpc ./internal/app -run 'Test(Goal|WorkAdmission|AppShutdownStopsGoalAdmissionBeforeChannelDrain)' -count=1` passed (`runtime` 12.122s, `rpc` 2.743s, `app` 5.489s), no race report.
- Source digest: `go run ./sdk/internal/cmd/source-hash internal ''` returned `2684c3a266f4e31d3448ea680f0adfca8d67c9ef5d96714b21485fd85d6481f5`, refreshed in the same five SDK conformance entries.
- The single full `just ci` for the corrected tree passed, exit 0: UI typecheck, 49 files/400 tests, build/i18n; Go vet and all packages (`internal/app` 121.784s, `internal/rpc` 289.530s, `internal/runtime` 470.151s, `sdk/internal` 698.011s); headless compile; plugin/face vet and tests. Docker and live PostgreSQL checks remain deferred by user instruction.
