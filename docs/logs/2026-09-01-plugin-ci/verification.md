# Verification

| Command | Result |
|---|---|
| Per-module baseline (manually ran `go vet && go test` before the change) | dingtalk/discord/feishu/lsp/telegram ok; **qq vet rejected the build** (go.mod updates needed) → the first run of the new gate caught real drift |
| `go mod tidy -diff` (plugins/qq, preview) | Indirect oauth2/gjson/pretty versions raised + stale go.sum lines removed |
| `cd plugins/qq && go mod tidy && go vet ./... && go test ./...` | go.mod/go.sum updated; `ok example.com/vivy/plugins/qq` |
| `gofmt -l` over `rg --files plugins -g '*.go'` | Empty (all files formatted); no regression after expanding the fmt-check glob |
| `just plugin-ci` (recipe run alone) | Six modules (dingtalk/discord/feishu/lsp/qq/telegram), each vet+test fully ok |
| `just ci` (full gate, including wired plugin-ci) | Green: fmt-check (new glob includes plugins) + vet + test + headless-compile + plugin-ci + ui-ci (tsc / 195 vitest / build) |

Failure-propagation evidence: the qq drift scenario itself failed vet → recipe
`$fail=1` → non-zero exit (demonstrated during the manually run baseline); test
failures use the same `$LASTEXITCODE` channel.

There were no UI or runtime behavior changes, so `just ui-e2e` does not apply to
this item (not rerun separately).
