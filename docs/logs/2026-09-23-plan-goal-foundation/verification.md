# PG-0 implementation and verification record

Date: 2026-09-23

Branch: `feat/issue-47-goal-plan-foundation`

## Implemented contracts

- Plan guidance is applied to the next model request. `submit_plan` durably records the proposal, then suspends the originating Eino run at its exact resume target. A human decision resumes that target once; replaying the same decision does not resume twice.
- The Plan suspension stores its originating run, tool call, resume target and same-batch sibling call IDs. Sibling effects remain fenced after restart and after the review decision. Restart recovery rebuilds only a readable, identity-matched pending review. Cancellation and leaving Plan close the pending run without silently resuming it.
- History messages and Work events use durable sequence anchors so same-timestamp ordering is deterministic across SQLite and PostgreSQL. Fork and rewind preserve the source history and do not refund spent Goal rounds or copy approval authority.
- Human turns are synchronous, process-local admission waiters. Intent registers before waiting on the per-session startup gate; no durable ticket or RunID exists before commit. A committed primary run returns the existing busy conflict. Cancellation before commit writes neither message nor run; after commit, normal detached-run lifetime applies.
- Shared Plan text, Goal objective and round limits are owned by `internal/domain` and consumed by tools, runtime and RPC validation.

## Scenario evidence

| Scenario | Command | Result |
| --- | --- | --- |
| Plan review fences same-batch calls, persists exact Eino resume identity, recovers in a second Service instance, resumes once and accepts replay idempotently | `PATH=/tmp/go/bin:$PATH go test ./internal/runtime -run '^TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch$' -count=1 -timeout 45s` | Passed |
| Human intent registers before the session gate; cancellation before admission leaves no run or message | `PATH=/tmp/go/bin:$PATH go test ./internal/runtime -run 'TestHumanAdmission|TestCancelledHumanAdmission' -count=1 -timeout 30s` | Passed |
| Domain lifecycle, runtime, RPC, SQLite, PostgreSQL adapter and migration regressions | `PATH=/tmp/go/bin:$PATH go test ./internal/runtime ./internal/domain ./internal/storage/sqlite ./internal/storage/postgres ./internal/storage/migrations ./internal/rpc -count=1 -timeout 5m` | Passed |
| History ordering, same-timestamp anchors, fork/rewind and spent-round behavior | Runtime rewind probes and storage conformance tests in the full suite | Passed |
| SDK provider conformance snapshot matches the source tree | `PATH=/tmp/go/bin:$PATH go test -p 4 -timeout 20m ./...` | Passed after updating the five `internal/` digests |

## Repository gates

| Gate | Result |
| --- | --- |
| `gofmt` on changed Go files and tracked-file format scan | Passed |
| `PATH=/tmp/go/bin:$PATH go vet ./...` | Passed |
| `PATH=/tmp/go/bin:$PATH go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | Passed |
| Full Go suite: `PATH=/tmp/go/bin:$PATH go test -p 4 -timeout 20m ./...` | Passed |
| UI typecheck, 48 test files / 392 tests, production build, catalog completeness and cross-face i18n checks | Passed; Vite reported the >500 KB main-chunk advisory |
| Independent `plugins/` and `faces/` module vet/tests | Passed |
| Live PostgreSQL test | Not run: `VIVY_POSTGRES_TEST_DSN` is unset |
| Browser E2E and real coding walkthrough | Not run in this continuation; no UI source behavior changed |

The container does not have `just`; its `ci` gates were run in recipe order as direct commands and passed. Live PostgreSQL, browser integration and a real coding walkthrough remain as broader acceptance gates.
