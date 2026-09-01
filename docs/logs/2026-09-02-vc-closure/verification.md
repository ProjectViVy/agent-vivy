# Verification

| Step | Command | Result |
|---|---|---|
| Merge conflict resolution build | `gofmt -l internal ui/src` (lane worktree, after resolve) + `go build ./...` | clean, build OK |
| Lane merge CI | `just ci` (vc0 worktree, post-merge) | CI-EXIT:0 |
| Lane merge UI e2e | `just ui-e2e` (vc0 worktree) | 13 passed |
| Lane fast-forward to main | `git merge feat/vc1a-bash-tool` (root) | fast-forward, `git log main..feat/vc1a-bash-tool` empty |
| rb1 containment | `git merge feat/rb1-rollback-research` (root) | "Already up to date"; `git log main..feat/rb1-rollback-research` empty |
| Root landing CI | `just ci` (root, post-fast-forward) | CI-EXIT:0 |
| Walkthrough test | `go test ./internal/runtime/ -run TestVC1Walkthrough -v` | PASS (1 test), assertions: no approval interrupt, 5 ordered tool.finished zero-error, diff present, file_versions chain baseline-empty → PASS, single run.completed |
| Regression after fixes | `go test ./internal/tools/ ./internal/storage/sqlite/ ./internal/runtime/` | PASS |
| Final slice CI | `just ci` (root, with walkthrough test + fixes + TODO) | CI-EXIT:0 |

Determinism note: TestVC1Walkthrough is offline (ScriptedModel replay + tempdir workspace + tempdir sqlite); no live network, no provider key.
