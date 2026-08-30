# CH-C2 — verification

日期：2026-08-30。工作区：worktree `agent-vivy-channel-c2`，分支 `feat/channel-c2`，基线 c811d9f（含 CH-C1）。

## GOAL 运行方式（子代理分工）

- explore（只读）：扫 sdk/plugin、verify/pack/CLI、pluginhost、config、zz_register 机制、独立 module 语义、PLUGIN-SPEC 与 CHANNEL-PACK 的符号差异。
- executor（写盘，cwd 锁定 C2 worktree）：按 CH-C2 文件清单实现 16 文件（11 改 + 5 新）。
- reviewer（只读独立评审）：结论 **PASS**，无 blocker，8 条 note（覆盖缺口与边角，均登记或说明）。

## 命令与结果（均在 C2 worktree 根执行）

| 命令 | 结果 |
|---|---|
| `gofmt -l ./sdk ./internal` | 无输出（干净，含 testdata） |
| `go test ./...`（executor 后） | 23 包 ok，零 FAIL |
| `go test ./sdk/... -v` | `TestPackFakeChannelStandaloneModule` PASS（7.7s，真实双 overlay `go build`）；verify 三条拒绝夹具 + `TestVerifyFakeChannel` PASS |
| `go test ./internal/config/... ./internal/pluginhost/... -v` | 信封测试 + `TestAdaptSkipsChannelSeam`（桩带非空 Tools()）全 PASS |
| `just ci`（首跑 vet 在 `ui/embed.go all:dist` 失败 = 已知 UI-CI-BOOTSTRAP；`cd ui && pnpm install --frozen-lockfile && pnpm build` 后重跑） | **exit 0**：Go 全部包 ok（sdk/internal 13.9s 含真实 pack 构建；runtime 26.7s），UI 21 文件 / 175 测试，vite build 绿 |
| reviewer 复核：`go run ./sdk verify sdk/internal/testdata/fake-channel` | `ok`，exit 0 |
| reviewer 复核：`verify bad-channel-tools` / `bad-channel-listen` / `bad-channel-grant` | 均 exit 1，issue 文案分别命中 tools 禁令 / listen 封禁 / 本批 grant 禁令 |
| reviewer 复核：`go list -deps ./... \| grep -iE "telego\|discordgo\|lark\|botgo\|pion"` | 空 |
| reviewer 复核：`git diff c811d9f -- go.mod go.sum` | 0 行 |
| `internal/generated/plugins/zz_register.go` 直读 | 仍 `return nil` |

## 验收清单（CH-C2.md §7）

- fake-channel `verify` 通过 ✅
- 带 tools 的 channel 清单失败 ✅
- 含 `net.Listen` 的源失败 ✅
- `just ci` 的 import 图无 telego 等 ✅
- 提交树 `zz_register.go` 仍 `return nil` ✅
- `pluginhost.Adapt([]Plugin{fakeChannel})` 长度为 0（桩证明非偶然）✅

## 诚实声明（环境限制 / 过程事件）

- executor 过程中误敲过一次 `git stash`，立即 `git stash pop` 恢复；review 与 `git status` 确认工作区仅含交付文件、stash 栈为空。
- executor 曾临时创建 `ui/dist/index.html` 以让 pack 构建通过，测完即删（gitignored 构建产物，未触碰任何 tracked 文件）；门禁用正规 `pnpm build` 产物复跑。
- verify 的四个分支（非 channel seam 领 channel 族 grant、channel 重复 grant、transport=webhook、负 max_message_runes）实现正确但暂无夹具——登记 `docs/TODO.md` §0.1（CH-C2-N1），C3 TCK 一并硬化。
- 未 push。
