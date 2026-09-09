# Verification — TUI-WORKSPACE-OWNERSHIP

Commands run from repository root (Windows, Git Bash):

| Command | Result |
|---|---|
| `go test ./internal/runtime/ -run 'Workspace' -count=1` | ok 0.544s |
| `go test ./internal/rpc/ -run 'Workspace' -count=1` | ok 4.015s |
| `go test ./internal/runtime/ ./internal/rpc/ -count=1` | ok 164.665s / 82.264s |
| `gofmt -l` on the four changed files | clean |
| `go vet ./internal/runtime/ ./internal/rpc/` | clean |
| `just ci` | exit 0 (all packages ok, including plugin-ci and UI build) |

New tests:

- `internal/rpc`: `TestWorkspaceRPCFailsClosedForUnknownRun` (unknown run → 404,
  with zero calls to the WorkspaceFiles stub),
  `TestWorkspaceRPCWorkspaceNotFoundIsNotFound` (`ErrWorkspaceNotFound` → 404).
- `internal/runtime`: `TestWorkspaceFilesUnknownRunFailsClosed` (unknown run →
  `ErrWorkspaceNotFound`; ids containing control characters / `..` / separators
  are all rejected; zero manager-root directory creation),
  `TestWorkspaceFilesReadSurvivesPathSwap` (final-component symlink rejected +
  ordinary reads remain unaffected after hardening).
