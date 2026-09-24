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
