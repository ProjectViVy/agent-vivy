# Verification

All commands run from the repository root (Windows, Git Bash), 2026-09-07.

## Builder-level (each subagent before handoff)

| Command | Result |
| --- | --- |
| `go build ./sdk/...` | pass |
| `go test ./sdk/tui/view/ -count=1` (verbose) | 102 `--- PASS`, 0 `--- FAIL` |
| `gofmt -l sdk/tui/view` | empty |
| `go vet ./sdk/tui/view/` | clean |

## Supervisor integration

| Command | Result |
| --- | --- |
| `go test ./sdk/tui/view/ -count=1` (combined tree incl. concurrent lane) | `ok agent-vivy/sdk/tui/view` |
| `gofmt -l sdk/tui/view sdk/tui/live sdk/tui/surface` | empty |
| `just ci` (fmt-check, ui-ci, vet, test, headless-compile, plugin-ci) | all green; headless build of `./cmd/vivy-code` passes |
| temp worktree @ `2768390` (F10 alone): `go build ./sdk/...` + `go test ./sdk/tui/view/` | pass |
| temp worktree @ `83d242e` (F7 on top): same | pass |

## Commits

- `2768390` feat(tui): color modified-file counts and middle-truncate sidebar paths
  (`render.go` F10 hunks only + `view_test.go` fixture + `modified_files_test.go`)
- `83d242e` feat(tui): set terminal window title from the active session
  (`model.go` window-title hunks only + `window_title_test.go`)

Hunk-scoped staging (`git apply --cached` on filtered diffs) kept a
concurrently active lane's chrome-timer work (`surface.go`, `controller.go`,
spinner/elapsed chrome hunks, `chrome_status_test.go`) out of both commits.
Each commit was verified to build and pass the view package independently in
a throwaway worktree before the next was cut.

## Smoke path and limitations

- Rendering is verified by in-process assertions against the real styled
  output (`p.DiffAdd.Render("+12")` / `p.DiffDel.Render("-3")` substrings,
  display-width fit at production sidebar width 32, filename tail visible).
- The terminal window title is a terminal-emulator side effect that cannot be
  screenshotted in this environment; it is covered by unit tests on the
  returned command and the sanitized title payload, and the product binary
  builds (`just ci` headless compile of `cmd/vivy-code`).
- A human should eyeball both behaviors in a live `vivy-code` session (see
  `acceptance.md`).
