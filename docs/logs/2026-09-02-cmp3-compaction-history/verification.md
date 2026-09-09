# Verification — CMP-3

Commands run from the repository root (`agent-vivy/`):

1. `go build ./...` — clean.
2. `go vet ./internal/storage/... ./internal/rpc/ ./internal/app/` — clean.
3. `go test ./internal/storage/sqlite/ ./internal/storage/postgres/ -run 'Conformance|CN-20' -count=1`
   — First run FAIL: the CN-20 tie expectation was reversed (`run-c0` was expected before
   `run-c2`, but `run_id DESC` put `run-c2` first). After correcting the fixture (renaming
   the tied record to `run-c9`), the sqlite CN-20 rerun was ok.
4. `gofmt -l internal/` — first run reported `internal/rpc/control.go` (ControlDeps
   struct alignment); clean after `gofmt -w`. The first `internal/rpc` test build failed:
   `control_test.go` was missing the `storage` import; it passed after the import was added.
5. `go test ./internal/rpc/ -run 'TestControlHandlerListsSessionCompactions' -count=1`
   — ok (0.9s).
6. `just ci` — full run in the background, with the log tail checked for `CI-EXIT:0`
   (see the verification addendum in this directory; checked before the final commit).
7. `just ui-e2e` — full run in the background, with the tail checked for `E2E-EXIT:0` and
   the two compaction-setting specs passing.

Note: as agreed, the postgres conformance suite skips execution when no DSN is available
(CN-20 postgres coverage relies on the existing postgres job behavior in the full `just ci`
environment; sqlite is fully green for this slice).
