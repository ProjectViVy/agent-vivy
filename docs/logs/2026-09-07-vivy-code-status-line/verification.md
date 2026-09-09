# Verification

All commands run from repo root, Windows / Git Bash, on the delivered tree
(content identical to commit `e2ad7f2`; the parallel window-title lane's
commits `2768390`/`83d242e`/`fc63529` are also present).

| Command | Result |
| --- | --- |
| `go build ./...` | OK |
| `go vet ./sdk/tui/...` | OK |
| `gofmt -l sdk/tui/` | clean |
| `go test -count=1 ./sdk/tui/view ./sdk/tui/live` | `ok … view 2.379s`, `ok … live 1.790s` |
| `go test -count=1 ./sdk/tui/...` (post-commit) | all `ok` (command/face/live/stream/view) |
| `just ci` | all steps passed (kernel tests, plugins/{dingtalk,discord,feishu,lsp,qq,telegram}, faces/headless, and faces/tui all ended with `ok`, exit 0) |
| `go build -tags vivy_headless -o vivy-code.exe ./cmd/vivy-code` | OK (≈117 MB binary) |
| `TUI_PREVIEW=1 go test ./sdk/tui/view -run TestDumpComposerPreview` | three rendered states confirmed (see below) |

## New deterministic tests

- `TestRenderInputChromeBusySpinnerElapsed` — busy chrome includes braille frames and `run 1m30s`
- `TestRenderInputChromeQueuedCount` — `Queued=2` displays `queued 2`; with 0, there is no queued text
- `TestRenderInputChromeScrollIndicator` — after 60 overflowing messages + `scrollChat(-10)`, the right segment contains `↓ ` and `end to bottom`

## TUI_PREVIEW real-render dump (real `View()` render path; outside a TTY, lipgloss strips light ANSI)

```
chrome[busy+queued]: ⠙ run 1m15s  shift+tab switch mode  shift+h help  ctrl+x shortcuts                    queued 2
chrome[scrolled]: shift+tab switch mode  shift+h help  ctrl+x shortcuts                         ↓ 90% · end to bottom
```

## Notes

- Interactive real-terminal smoke cannot be automated in this environment (no
  TTY), so full-frame `TUI_PREVIEW` rendering plus a successful
  `vivy-code.exe` build was used instead; see `acceptance.md` for the human
  steps.
- Some existing `sdk/tui/live` tests set `l.busy` directly (without going
  through `setBusyLocked`), producing a zero-value `BusySince` → chrome shows
  `⠋ run` (no elapsed time). This is the in-spec zero-timestamp behavior; all
  7 production-path transitions converge on the helper.
