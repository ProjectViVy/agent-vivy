# CH-C4 verification

日期：2026-08-30。全部命令在 worktree
`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-channel-c2`
（分支 `feat/channel-c4`）根目录执行。

| 命令 | 结果 |
|---|---|
| `go build ./...` && `go vet ./...` | PASS |
| `go list -deps ./cmd/vivy \| grep -c telego` | `0`（默认身体无 telego） |
| `go test ./...`（物种根） | PASS；含 channelhost TCK 全套 + 新 env 6 项 + sdk pack 全套（含真实 e2e `TestPackTelegramStandaloneModule`）；telego 闭包不在物种编译图 |
| `gofmt -l ./internal ./sdk ./plugins/telegram` | 无输出 |
| `just ci`（fmt-check / vet / test / headless-compile / ui-ci） | PASS（ui 175 tests、vite build 成功） |
| `go run ./sdk verify plugins/telegram` | `ok ...\plugins\telegram` |
| `go run ./sdk pack --with telegram --out /tmp/vivy-c4-out` | `gen_907fa3d138e87e30`，`recipe.plugins=["telegram"]`，phase `built` |
| `go run ./sdk inspect-artifact /tmp/vivy-c4-out` | 与 generation.json 一致；候选 EXE 内 `github.com/mymmrac/telego` 出现 7 次（telego 闭包链接成功） |
| `git diff -- go.mod go.sum` | 空（字节不变，站立命令遵守） |
| `cd plugins/telegram && go vet ./... && go test ./... -count=1` | PASS（4.3s；httptest 回环，无真 Telegram 网络） |
| `go test ./internal/channelhost/ ./sdk/internal/ -count=1`（重复跑） | PASS（稳定，无 flake） |

跳过项：CH-C4.md §6.8 真机冒烟（真 Bot + 非空 allow_from）为可选项，本
切片无真 bot token，未执行；步骤见 acceptance.md 的手工脚本，属发布前
人工验收。

## 环境耦合（诚实声明）

- `TestPackTelegramStandaloneModule` 会真实构建候选 EXE：冷机器上首次运行
  会经 go module proxy 下载 telego 闭包（本环境 pnpm install 同样走网络，
  ci 网络可用；模块缓存后离线稳定）。该测试还把 `telego v1.10.0` 写进了
  断言文案——升级插件 telego 版本时需同步改断言。
- telego 默认 logger 自带 token 脱敏（`BOT_TOKEN` 替换），已由 reviewer
  对 telego@v1.10.0 源码核实（logger.go:46-54）。
- `justfile` 的 `fmt-check` 只扫 `cmd internal sdk ui`，plugins/telegram
  的 gofmt 由本切片人工执行（`gofmt -l plugins/telegram` 无输出）。
