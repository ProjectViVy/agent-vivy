# CH-C5 — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c2` (reused sequentially),
branch `feat/channel-c5`, baseline 5374d6f (including C1–C4).

## GOAL execution (subagent roles)

- explore (read-only): scanned RPC dispatch/ControlDeps, the settings overlay
  mechanism (the only runtime persistence path), Host inspect gaps, and the UI channel
  trio + i18n + test/e2e layout.
- executor (writes): 23 files (four Go packages + thirteen UI files + tests).
- reviewer (independent read-only review): **PASS**, no blocker; two should-fix items
  (log/TODO and branch name) were handled as landing steps.
- Browser smoke: run by the GOAL owner (smoke-for-user-visible-change).

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l ./internal ./sdk ./cmd` / `go build ./...` / `go vet ./...` | Clean (executor) |
| `go test ./...` (executor) | All green |
| `just ci` (executor) | Green: ui-ci 21 files / 172 tests (3 fewer than 175 because email/neuro-link platform cases were removed) + typecheck + vite build |
| reviewer recheck `go test ./internal/{rpc,channelhost,app,app/settings}/... -count=1` | All ok |
| reviewer recheck `pnpm vitest run` (channel-schema/store/api/i18n) | 41/41 pass |
| Secret-surface grep (reviewer) | The full RPC surface contains only `TokenEnv` (name) + `TokenEnvSet` (bool); `token_value` does not exist |
| localStorage grep (reviewer) | Zero read residue for `vivy.ui.channels` |
| i18n dead-key bidirectional grep (reviewer) | 12 keys removed, zero dangling references |

## Browser smoke (real path, not screenshot acceptance)

| Step | Result |
|---|---|
| `just run` (default body 8787) + `pnpm dev` (3015) + open `/settings?tab=channels` | Empty state "This generation has no ears" + guidance; no Add button or email/neuro-link; screenshot archived |
| `go run ./sdk pack --with telegram --out <scratch>` → candidate EXE + scratch config (mock provider; `channels.telegram` enabled + allow_from + token_env in both places; env deliberately unset) | Candidate starts, telegram card visible (enabled/needs configuration), `start failed: …` reason shown verbatim = full fail-closed chain observable (settings decode → envelope name pinning → Secret env resolution failure) |
| Editor | "one sender per line; empty = refuse startup" + token_env name + "unset" badge + D-010 explanation |
| Clear allow_from and save | `settings.yaml` persists `allow_from: []` (refused-start semantics) |
| Restore the two-line allowlist and save | Overlay updates to two lines (full UI→RPC→settings.yaml chain) |
| Narrow viewport 375×720 | Cards/controls normal, no horizontal overflow |

Smoke environment note: while starting the candidate, a root-tree Vite dev server
occupying 3015 was found and cleared (it served old code and would contaminate the
smoke); scratch config/data stayed in system temporary directories, with the air gap
untouched; processes and scratch were all cleared afterward.

## Scope guard

- All 24 modified files are within the CH-C5 §5 directions (rpc / app / app/settings /
  channelhost / UI channel surface / i18n); `internal/config`, plugins,
  `zz_register.go`, and go.mod were untouched.
- Not pushed.
