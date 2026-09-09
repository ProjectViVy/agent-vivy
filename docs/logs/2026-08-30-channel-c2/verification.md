# CH-C2 — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c2`, branch `feat/channel-c2`,
baseline c811d9f (including CH-C1).

## GOAL execution (subagent roles)

- explore (read-only): scanned sdk/plugin, verify/pack/CLI, pluginhost, config,
  zz_register mechanics, independent-module semantics, and symbol differences between
  PLUGIN-SPEC and CHANNEL-PACK.
- executor (writes, cwd locked to the C2 worktree): implemented the CH-C2 file list,
  16 files (11 modified + 5 new).
- reviewer (independent read-only review): **PASS**, no blocker, 8 notes (covering gaps
  and edge cases, all recorded or explained).

## Commands and results (all run at the C2 worktree root)

| Command | Result |
|---|---|
| `gofmt -l ./sdk ./internal` | No output (clean, including testdata) |
| `go test ./...` (after executor) | 23 packages ok, zero FAIL |
| `go test ./sdk/... -v` | `TestPackFakeChannelStandaloneModule` PASS (7.7s, real two-file overlay `go build`); three verify rejection fixtures + `TestVerifyFakeChannel` PASS |
| `go test ./internal/config/... ./internal/pluginhost/... -v` | Envelope tests + `TestAdaptSkipsChannelSeam` (stub with non-empty Tools()) all PASS |
| `just ci` (first vet failed at `ui/embed.go all:dist` = known UI-CI-BOOTSTRAP; rerun after `cd ui && pnpm install --frozen-lockfile && pnpm build`) | **exit 0**: all Go packages ok (sdk/internal 13.9s including real pack build; runtime 26.7s), UI 21 files / 175 tests, vite build green |
| reviewer recheck: `go run ./sdk verify sdk/internal/testdata/fake-channel` | `ok`, exit 0 |
| reviewer recheck: `verify bad-channel-tools` / `bad-channel-listen` / `bad-channel-grant` | all exit 1, with issue copy matching the tools prohibition / listen ban / this-batch grant ban respectively |
| reviewer recheck: `go list -deps ./... \| grep -iE "telego\|discordgo\|lark\|botgo\|pion"` | empty |
| reviewer recheck: `git diff c811d9f -- go.mod go.sum` | 0 lines |
| direct read of `internal/generated/plugins/zz_register.go` | still `return nil` |

## Acceptance checklist (CH-C2.md §7)

- fake-channel `verify` passes ✅
- channel manifest with tools fails ✅
- source containing `net.Listen` fails ✅
- `just ci` import graph has no telego, etc. ✅
- submitted-tree `zz_register.go` remains `return nil` ✅
- `pluginhost.Adapt([]Plugin{fakeChannel})` has length 0 (stub proves it is not accidental) ✅

## Honest statement (environment limits / process events)

- During executor work, `git stash` was accidentally run once and immediately
  recovered with `git stash pop`; review and `git status` confirmed that the
  worktree contained only deliverable files and the stash stack was empty.
- The executor temporarily created `ui/dist/index.html` so pack could build, then
  deleted it after testing (a gitignored build artifact; no tracked file was touched);
  the gate was rerun using the proper `pnpm build` output.
- Four verify branches (a non-channel seam claiming a channel-family grant, duplicate
  channel grant, `transport=webhook`, and negative max_message_runes) are implemented
  correctly but have no fixtures yet—recorded in `docs/TODO.md` §0.1 (CH-C2-N1), with
  C3 TCK hardening.
- Not pushed.
