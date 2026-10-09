# Verification

Environment: Linux, Go 1.26.4, pnpm 11.25.0, pinned Laputa
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.
The Go toolchain directory was added to PATH for each command.

| Command or check | Result |
| --- | --- |
| `go test ./internal/rpc -count=1` before merging main | Pass |
| `GOFLAGS=-tags=vivy_headless go test -race ./internal/rpc ./sdk/facerun -count=1` | Pass; both complete packages, no race reports |
| `GOFLAGS=-tags=vivy_headless go vet ./internal/rpc ./sdk/facerun` | Pass |
| `GOFLAGS=-tags=vivy_headless go test ./internal/runtime -run 'TestBashBackendQuoted.*' -count=1` | Pass; included in an invocation also selecting the tools package |
| `GOFLAGS=-tags=vivy_headless go test ./internal/tools -run 'TestClassifyShellScript(Tiers\|RejectsSyntaxErrors)' -count=1` | Pass |
| `pnpm install --frozen-lockfile` and `pnpm build` in `ui/` | Pass; no tracked UI changes |
| `go test ./sdk/internal -run 'TestGenerationFailureMatrixExecutesEveryCase/module-conflict' -count=1` | Pass |
| `GOFLAGS=-buildvcs=false go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1 -v` | Pass; all 23 release-suite cases, including selected UI packing and inspection |
| `go run ./sdk/internal/cmd/source-hash internal 6ec575e0a87d125980d51ee8296e2a4a60a1e0619520d55de6d89ec875dd606f` | Returns the same digest |
| Diff of RPC source/tests against PR head, and bash source/tests against main | Empty; both implementations preserved |
| `git diff --cached --check`; unresolved index entries | Pass; no unresolved entries |

The first conformance reproduction attempt confirmed the refreshed internal
digest but failed the UI packing suites because this fresh worktree had no
`ui/dist`. Built the UI before repeating the producer gate.

The next attempt passed all backend suites but UI packing failed while Go
obtained VCS status. `go build -x -n ./cmd/vivy` showed that Go was invoking
`git status` in `/workspace`, rather than this linked worktree. Removing the
global Git config did not fix it. Repeating the build dry run with
`GOFLAGS=-buildvcs=false` passed. This setting affects only automatic build
revision metadata; source hashing, conformance execution, and artifact
inspection remain enabled. The final reproduction uses that environment
setting without changing repository code or task recipes.

## Full CI limitation

Attempted `just ci`; this environment has no `just` executable (exit 127).
The repository's task recipes also use PowerShell. Full local CI is therefore
not claimed. Existing PR CI run
[37733241769](https://github.com/ProjectViVy/agent-vivy/actions/runs/37733241769)
had already failed before this resolution: Windows backend application tests,
a 35-minute runtime timeout, a foreground-job cancellation test, and codeclient
executable lookup. Its UI and browser-smoke lanes passed. Those failures were
not changed as part of this merge-conflict repair.
