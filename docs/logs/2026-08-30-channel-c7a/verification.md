# 验证记录

全部命令在 worktree `agent-vivy-channel-c2`（branch `feat/channel-c6`，
HEAD `398b273`）根目录执行：

| 命令 | 结果 |
|---|---|
| `gofmt -l plugins/feishu internal sdk cmd` | 空（全部已格式化；justfile fmt-check 不覆盖 plugins/，已手工覆盖） |
| `go build ./...`（根） | OK |
| `go vet ./...`（根） | OK |
| `go test ./...`（根） | 24 个包 ok，0 fail |
| `go vet ./...`（plugins/feishu） | OK |
| `go test ./... -count=1`（plugins/feishu） | ok 3.3s（12 个测试 + 子测试全过） |
| `go test -race ./... -count=1`（plugins/feishu） | ok 4.3s，无数据竞争、无 goroutine 泄漏告警 |
| `go run ./sdk verify plugins/feishu` | `ok ...\plugins\feishu` |
| `go run ./sdk pack --with feishu --out <tmp>` | 产出 `gen_56765f958c7b19db` 候选 EXE，`recipes.plugins` 含 `feishu`（inspect-artifact 确认） |
| `git diff -- go.mod go.sum` | 空——物种 go.mod/go.sum 零改动 |
| `go list -deps ./cmd/vivy \| grep -c "larksuite\|lark"` | `0`——lark SDK 只活在插件模块 |
| `GOARCH=386 go build ./...`（plugins/feishu） | 失败（`math.MaxInt64` overflow）——确认 64 位硬约束 |

测试网络约束：无任何真实飞书 / Lark 网络触达。全部平台交互落在
httptest 环回桩上（WS bootstrap、WS 帧、tenant_access_token、
im.v1.messages），WS 帧编解码使用 SDK 自身的 `larkws.Frame`
Marshal/Unmarshal，RFC6455 服务端握手 / 帧读写为标准库手写
（gorilla/websocket 保持为 SDK 的 indirect 依赖，不进插件 go.mod 的
直接依赖）。测试凭据全合成，且 app id 逐测试唯一（SDK 的 tenant
token 缓存是包级单例、按 app id 键控，唯一 id 防止测试间共享缓存）。
