# Verification — provider agentic spike

Environment: linux/amd64 cloud sandbox, Go 1.26.4, running as root. All Go
work ran in throwaway copies under the session scratchpad; the repository's
`go.mod`, `go.sum` and `internal/` were never written.

## S0

| Command (per baseline M = `e7dcf85`, B = `a5f1f90`) | Result |
|---|---|
| `git archive <commit>` + sibling `laputa` worktree at the pinned commit | ok |
| `go list -m all` on the raw copy | 346 modules each |
| ctl: `go mod tidy -e && go list -m all` | 342 each |
| add: blank import + `go get eino@v0.9.21 agenticopenai@v0.2.4 agenticclaude@v0.1.7` + tidy | 352 each |
| rep: add minus 5 old-component imports + tidy | 350 each |
| delta classifier ctl→add, ctl→rep | +10/−0/~5 and +10/−2/~5, identical for M and B |
| `go mod why -m` on added modules (rep) | all via `agenticopenai` → `openai-go/v3/azure`; `azidentity` test-only |
| `go list -deps ./internal/provider` (add) | `azcore`, `sdk/internal`, `openai-go/v3` linked |
| `go build ./...` (ctl, add; with placeholder `ui/dist`) | exit 0 on M and B |
| `go build -ldflags="-s -w" ./cmd/vivy` (ctl, add) | sizes in S0 §7.5 |
| probe module: `go build -ldflags="-s -w"` for `eino`, `current`, `replacement` | 11,641,097 / 34,009,353 / 39,194,889 B |
| `go test ./internal/provider/... ./internal/runtime/...` (ctl, add; M and B) | only `TestDeliverableReadSnapshotWriteFailure` fails, on all four; it expects an unwritable directory, which root defeats (`deliverable_service_test.go:562`) |

## S1

Module-cache reads at the pinned versions (cited inline in S1 §9); the
bridge-search command and output are verbatim in S1 §9.2. `rg` over `main` for
ADK/compose symbols, middleware sites and transfer usage.

## S2

| Command | Result |
|---|---|
| `go vet ./...` (spike module) | clean |
| `go test -race -count=1 -v -run 'TestPlainReply\|TestToolRoundTrip\|TestFailures\|TestIsolation\|TestParallelEnhancedTools\|TestClaudeUsageCumulativeDelta'` | 21 result lines: PASS except the findings in S2 §9.2 (Claude usage total ×2 cases, Responses replay, truncated ×3); 0 `DATA RACE` |
| `go test -count=1 -v -run TestDeferredTools` | 3 PASS; wire shapes in S2 §9.2 |
| `go test -race -count=1 -run TestTruncatedCurrentGeneration` | `DATA RACE` in `components/model/claude@v0.1.25 claude.go:446` |
| current-generation module at `main`'s exact pins (eino v0.9.13, openai v0.1.13, acl/openai v0.1.17, go-openai v0.1.2, claude v0.1.25): reasoning, truncation, user content shape | reasoning surfaced on both; truncation silent on both; user content sent as a string |

## S3

`rg -c` / `rg -l` counts and consumer searches over `main` `e7dcf85`, as
listed in S3 §7.1 and §7.3; the same counts on `a5f1f90` to explain the plan's
145/24 figure.

## Repository gates

Docs-only change (no product behavior or contract change):

- `git diff --check` — clean.
- `git diff e7dcf85 --stat` (upstream `main`) — only `docs/` paths.
- `just ci` not run: docs-only per `AGENTS.md` "Validation". `just` is also
  not installed in this sandbox.

## Skipped

- Live-provider calls (no credentials; B6/B7 need them with owner approval).
- Windows binary sizes.
- Product-path end-to-end checks (they belong to the migration's P4).
