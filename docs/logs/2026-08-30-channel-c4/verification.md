# CH-C4 verification

Date: 2026-08-30. All commands ran at the root of worktree
`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-channel-c2`
(branch `feat/channel-c4`).

| Command | Result |
|---|---|
| `go build ./...` && `go vet ./...` | PASS |
| `go list -deps ./cmd/vivy \| grep -c telego` | `0` (the default body has no telego) |
| `go test ./...` (product root) | PASS; includes the full channelhost TCK + 6 new env cases + full sdk pack suite (including real e2e `TestPackTelegramStandaloneModule`); telego closure is not in the product build graph |
| `gofmt -l ./internal ./sdk ./plugins/telegram` | No output |
| `just ci` (fmt-check / vet / test / headless-compile / ui-ci) | PASS (175 UI tests, vite build succeeded) |
| `go run ./sdk verify plugins/telegram` | `ok ...\plugins\telegram` |
| `go run ./sdk pack --with telegram --out /tmp/vivy-c4-out` | `gen_907fa3d138e87e30`, `recipe.plugins=["telegram"]`, phase `built` |
| `go run ./sdk inspect-artifact /tmp/vivy-c4-out` | Matches generation.json; `github.com/mymmrac/telego` appears 7 times inside the candidate EXE (telego closure linked successfully) |
| `git diff -- go.mod go.sum` | Empty (bytes unchanged, standing command obeyed) |
| `cd plugins/telegram && go vet ./... && go test ./... -count=1` | PASS (4.3s; httptest loopback, no real Telegram network) |
| `go test ./internal/channelhost/ ./sdk/internal/ -count=1` (repeat run) | PASS (stable, no flake) |

Skipped: the CH-C4.md §6.8 real-device smoke (real Bot + non-empty allow_from) is
optional; this slice had no real bot token, so it was not run. The steps are in the
manual script in acceptance.md and belong to pre-release human acceptance.

## Environment coupling (honest statement)

- `TestPackTelegramStandaloneModule` really builds a candidate EXE: the first run
  on a cold machine downloads the telego closure through the Go module proxy (this
  environment's pnpm install also uses the network; CI networking is available; it is
  stable offline after module caching). The test also hard-codes `telego v1.10.0`
  in its assertion copy—update the assertion when the plugin telego version changes.
- telego's default logger redacts tokens (`BOT_TOKEN` replacement), verified by
  the reviewer against telego@v1.10.0 source (logger.go:46-54).
- `justfile`'s `fmt-check` scans only `cmd internal sdk ui`; gofmt for
  plugins/telegram was run manually in this slice (`gofmt -l plugins/telegram` had
  no output).
