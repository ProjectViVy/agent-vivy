# Verification

All commands below ran on Windows in the isolated PR #82 repair checkout.

- Red regression: `go test ./internal/storage/sqlite -run '^TestIssue58UnresolvedToolRunIsSealedExcluded$' -count=1`
  failed before the fix because caller-supplied exclusion returned nil.
- Final focused test with `excluded`, `included`, and unknown caller dispositions
  — PASS, 3.607s. It also proves rejected input does not consume a sequence and
  exact replay of a storage-derived exclusion still succeeds.
- `go test -count=1 ./internal/storage/...` — PASS; SQLite package 371.974s.
  This run preceded the final guard broadening from `excluded` to all non-empty
  dispositions; the focused test above and race suite below ran after it.
- `go test -count=1 -timeout=10m ./internal/runtime -run 'Recap|Subject'`
  — PASS, 42.340s after the final guard.
- `go test -race -count=1 -timeout=10m ./internal/storage/sqlite -run 'TestIssue58UnresolvedToolRunIsSealedExcluded|Subject|Recap'`
  — PASS, 41.246s after the final guard.
- `go test -race -count=1 -timeout=10m ./internal/runtime -run 'Recap|Subject'`
  — PASS, 67.397s after the final guard.
- `git diff --check` — PASS.

The fork PR #82 remains open and is not writable by this repository. The owner
replacement PR will run its own GitHub checks; the fork's previous
`action_required` run is not treated as a pass. Final combined #79/#82
integration, current-main conflict reconciliation, and repository-wide CI are
still pending.
