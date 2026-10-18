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

## PR #85 follow-up candidate

The checks below ran locally on native Windows in the isolated replacement
branch after repairing the cross-face key classification, browser launch, and
Laputa dependency pin. They do not turn the previous remote run into a pass.

- `just ui-ci` — PASS; TypeScript, 743 UI tests, Vite build, translation
  completeness, and cross-face contract all completed.
- With a fresh `GOMODCACHE` under `%TEMP%\pr85-clean-gomodcache`,
  `go test -p=1 -count=1 -timeout=15m ./...` in `bml/` — PASS;
  `go test -mod=readonly -p=1 -count=1 -timeout=15m ./actmem` in
  `../laputa/laputa/` — PASS; and
  `go test -mod=readonly -p=1 -count=1 -timeout=15m ./internal/ingest ./internal/runtimecore ./agentapi`
  in `../laputa/garden/` — PASS. The clean cache fetched official
  `github.com/coder/hnsw v0.6.1`; Garden compiled and executed with Laputa
  `ccf0e253bde3b9297c2f287cfe1a809813e9aabc` and its declared local shim.
- `pnpm --dir ui exec playwright test --config playwright.issue58.config.ts`
  — PASS; both actual split-backend Recap browser tests passed (49.1s).
- `go test -race -count=1 -timeout=10m ./internal/storage/sqlite -run 'TestIssue58UnresolvedToolRunIsSealedExcluded|Subject|Recap'`
  and `go test -race -count=1 -timeout=10m ./internal/runtime -run 'Recap|Subject'`
  — PASS on native Windows (31.446s and 74.886s).
- `go test -race -count=1 -timeout=10m ./internal/modules/diva-cognitive ./internal/modules/memory`
  — PASS on native Windows (8.100s and 3.715s), including canonical path and
  owner cleanup regressions.
- From a temporary empty Go module with a fresh module cache,
  `go mod download -json` for Garden, Laputa, and Mentle at
  `v0.0.0-20261018203522-ccf0e253bde3` — PASS; the proxy metadata confirms
  all three resolve to the full `ccf0e253bde3b9297c2f287cfe1a809813e9aabc`
  commit. Their module and go.mod hashes are recorded in `go.sum`.
- `git diff --check` — PASS after the follow-up edits.

The old GitHub run [39158192360](https://github.com/ProjectViVy/agent-vivy/actions/runs/39158192360)
tested old head `83748fbaafa8535b02a270eb0bcbaa5e963edcea`: backend passed;
UI failed because `approvals.quick.otherSession` was unclassified; browser smoke
could not resolve the pnpm shim from Playwright's server process; and the
Garden packages failed to compile against old Laputa pin `27da7b9`. The fixes
above are local evidence for a new candidate only. GitHub CI for that new SHA
is still pending.
