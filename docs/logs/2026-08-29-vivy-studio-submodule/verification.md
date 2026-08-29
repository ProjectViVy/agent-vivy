# verification — vivy-studio submodule migration

## Commands and results

| Check | Result |
|---|---|
| Push `ProjectViVy/vivy-studio` `main` | PASS — `0699f77` initial, then `b7de607` docs |
| `git ls-remote …/vivy-studio.git HEAD` | `b7de607…` |
| Host: `git ls-files -s studio` | `160000 b7de607… studio` (single gitlink) |
| Host: `git ls-files studio` count | `1` (no vendored blobs) |
| `.gitmodules` has `path = studio` → vivy-studio | PASS |
| `scripts/ensure-studio.ps1` no-op when present | exit 0 |
| Cold start: `git submodule deinit -f studio` + wipe → ensure | re-checked out `b7de607`; sentinel `studio/dsh-vivy-studio/package.json` present; all 7 package dirs restored |
| `just ensure-studio` | exit 0 |
| `go build -o vivy-studio.exe ./cmd/vivy-studio` | exit 0 (CLI remains in host) |
| `just ci` on worktree base | **FAIL (pre-existing)** — `internal/app` references undefined `ModelResolver` / `TakeOrganismLease` on this HEAD; **no `.go` files in this deliverable commit** |
| `go test ./internal/studiocore` | FAIL for same baseline (eval builds vivy via broken `internal/app`) |
| Port 3090 | already in use by existing Studio on this machine; full relaunch smoke deferred to avoid killing the agent host |

## Notes

- Deliverable developed in isolated worktree
  `../agent-vivy-studio-submodule` on branch `feat/vivy-studio-submodule`
  per parallel-worktree-isolation (root tree was dirty with unrelated lanes).
- `just ci` gate not green on this worktree **baseline**; failure is orthogonal
  to submodule migration. Re-run `just ci` after merging a green species HEAD
  or on root once ModelResolver lane lands.
