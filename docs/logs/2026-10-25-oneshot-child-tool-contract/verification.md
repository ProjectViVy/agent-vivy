# Verification — one-shot child tool contract wording

Host: macOS (arm64), Go toolchain pinned with `GOTOOLCHAIN=go1.26.4`, tests
serial (`-p 1 -count=1`). Worktree: `.worktrees/oneshot-child-tool-contract`,
branch `docs/oneshot-child-tool-contract`, based on `cc3b66f3` (child workspace
redirect).

## Commands and results

| Command | Result |
| --- | --- |
| `git diff --check` | clean (exit 0) |
| `gofmt -l internal schemas docs` | no files listed |
| `GOTOOLCHAIN=go1.26.4 go vet ./...` | pass |
| `GOTOOLCHAIN=go1.26.4 go build ./...` | pass (after copying the gitignored `ui/dist` into this fresh worktree; `ui/embed.go` embeds `all:dist`) |
| `GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/tools/ ./internal/app/` | pass (`internal/tools` 3.0s, `internal/app` 19.3s) |
| `GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/ -run 'Child\|Agent\|Approval'` | pass (13.2s) |

The runtime mechanism is unchanged by this delivery; the focused runtime run
covers the child admission, approval, and workspace tests that surround the
edited surfaces. The full `./internal/...` suite was last green twice at
`cc3b66f3` and is not re-run for a prompt string and doc prose.

## Not run, and why

- `just wiring-ci` / `just kernel-ci` / `just d02-ci`: require an independent
  Linux/WSL checkout with its own pinned Laputa. macOS cannot certify them.
  Owner to run on the delivery lane.
- PostgreSQL-backed conformance: no DSN on this host; unchanged by this
  delivery (no storage code touched).
- Browser smoke at `http://127.0.0.1:3015`: no UI behavior changed by this
  delivery. The only model-facing change is the `agent` tool description
  string, which is not observable in the UI without a live provider turn; the
  capability it now mentions (a child reading parent-written files) was already
  verified end-to-end in `2026-10-25-child-shared-workspace`. Recording that
  sentence as unit-verified only (build + package tests), not model-verified:
  no provider key was used for a live delegation turn in this delivery.
