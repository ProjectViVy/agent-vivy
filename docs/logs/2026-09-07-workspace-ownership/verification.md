# Verification — TUI-WORKSPACE-OWNERSHIP

Commands run from repository root (Windows, Git Bash):

| Command | Result |
|---|---|
| `go test ./internal/runtime/ -run 'Workspace' -count=1` | ok 0.544s |
| `go test ./internal/rpc/ -run 'Workspace' -count=1` | ok 4.015s |
| `go test ./internal/runtime/ ./internal/rpc/ -count=1` | ok 164.665s / 82.264s |
| `gofmt -l` on the four changed files | clean |
| `go vet ./internal/runtime/ ./internal/rpc/` | clean |
| `just ci` | exit 0（全部包 ok，含 plugin-ci 与 UI 构建） |

New tests:

- `internal/rpc`: `TestWorkspaceRPCFailsClosedForUnknownRun`（unknown run → 404，
  WorkspaceFiles stub 零调用）、`TestWorkspaceRPCWorkspaceNotFoundIsNotFound`
  （`ErrWorkspaceNotFound` → 404）。
- `internal/runtime`: `TestWorkspaceFilesUnknownRunFailsClosed`（未知 run →
  `ErrWorkspaceNotFound`；控制字符/`..`/分隔符 id 全被拒；manager root 目录零创建）、
  `TestWorkspaceFilesReadSurvivesPathSwap`（final-component symlink 拒绝 + 硬化后
  常规读取不受影响）。
