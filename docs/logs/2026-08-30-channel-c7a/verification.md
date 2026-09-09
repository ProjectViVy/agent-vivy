# Verification record

All commands ran at the root of worktree `agent-vivy-channel-c2` (branch
`feat/channel-c6`, HEAD `398b273`):

| Command | Result |
|---|---|
| `gofmt -l plugins/feishu internal sdk cmd` | Empty (all formatted; justfile fmt-check excludes plugins/, so covered manually) |
| `go build ./...` (root) | OK |
| `go vet ./...` (root) | OK |
| `go test ./...` (root) | 24 packages ok, 0 fail |
| `go vet ./...` (plugins/feishu) | OK |
| `go test ./... -count=1` (plugins/feishu) | ok 3.3s (12 tests + all subtests passed) |
| `go test -race ./... -count=1` (plugins/feishu) | ok 4.3s, no data races or goroutine-leak warnings |
| `go run ./sdk verify plugins/feishu` | `ok ...\plugins\feishu` |
| `go run ./sdk pack --with feishu --out <tmp>` | Produced candidate EXE `gen_56765f958c7b19db`; `recipes.plugins` contains `feishu` (confirmed by inspect-artifact) |
| `git diff -- go.mod go.sum` | Empty—the product go.mod/go.sum have zero changes |
| `go list -deps ./cmd/vivy \| grep -c "larksuite\|lark"` | `0`—the lark SDK exists only in the plugin module |
| `GOARCH=386 go build ./...` (plugins/feishu) | Failed (`math.MaxInt64` overflow)—confirms the 64-bit hard constraint |

Test network constraint: no real Feishu / Lark network was contacted. All platform
interactions ran against an httptest loopback stub (WS bootstrap, WS frames,
tenant_access_token, im.v1.messages); WS frame encoding/decoding used the SDK's own
`larkws.Frame` Marshal/Unmarshal, while the RFC6455 server handshake/frame I/O
was hand-written with the standard library (gorilla/websocket remains an indirect SDK
dependency and is not a direct dependency in the plugin go.mod). Test credentials
were synthetic, and each test used a unique app id (the SDK's tenant-token cache is a
package-level singleton keyed by app id; unique IDs prevent cache sharing between
tests).
