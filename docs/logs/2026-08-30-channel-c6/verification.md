# CH-C6 — verification

日期：2026-08-30。工作区：worktree `agent-vivy-channel-c2`（顺序复用），分支 `feat/channel-c6`，基线 d3d4342（含 C1–C5）。

## GOAL 运行方式（子代理分工）

- explore：已由 C4/C5 铺垫（telegram 模板 + Host 面），本刀直接用 picoclaw 对照。
- executor（写盘）：首轮因超时中断（只留 go.mod/go.sum 骨架），重派新 executor 完成全部清单。
- reviewer（只读独立评审）：**PASS**；2 条代码级 should-fix（Stop 后迟到回调栅栏；replyText 解析错误路径 webhook token 泄漏 D-010）已由 executor 修复并加测试。
- 无浏览器冒烟需求（无 UI 改动）；真实钉钉组织冒烟属发布前人工验收（无凭据）。

## 命令与结果

| 命令 | 结果 |
|---|---|
| `gofmt -l`（internal / sdk / plugins/dingtalk——justfile fmt-check 不覆盖 plugins/，人工执行） | 干净 |
| 根 `go build ./...` / `go vet ./...` / `go test ./...` | 24 包 ok；钉钉 SDK 不在物种编译图（`go list -deps ./cmd/vivy \| grep -c dingtalk` = 0） |
| `git diff d3d4342 -- go.mod go.sum` | 空 |
| `cd plugins/dingtalk && go vet && go test ./... -count=1` | 10 测试全 PASS（含真实 SDK 回环 `TestStreamLoopbackLifecycle`、停止栅栏 `TestHandlerFencedAfterStop`、D-010 双阶段脱敏断言） |
| `go test -race ./... -count=1` | ok（1.2s） |
| `go run ./sdk verify plugins/dingtalk` | ok |
| `go run ./sdk pack --with dingtalk --out <tmp>` + `inspect-artifact` | 候选 EXE 链接 SDK（UserAgent/网关域名串在），recipe.plugins=["dingtalk"] |
| `go test ./sdk/... -count=1`（4 个新夹具） | ok |
| `go test ./internal/channelhost/... -count=1`（Secret 扩展回归 + telegram 语义不变） | ok |
| `just ci` | **exit 0**：Go 全部包 ok（channelhost 9.9s、rpc 20.4s、runtime 41.3s、sdk/internal 9.7s），UI 21 文件 / 172 测试，vite build 绿 |
| `git diff -- go.mod go.sum`（最终） | 空 |

## 验收清单（CH-C6.md §7）

- 候选单聊文本闭环：回环测试证明（ticket→握手→CALLBACK→入站→sessionWebhook 回复→Stop 不重拨）✅
- 默认身体无钉钉依赖 ✅
- 空 allow_from 拒绝（Host 行为，回归绿）✅
- 无公网 webhook 模式（transport 仅 poll；无 Listen；无 webhook grant）✅

## 诚实声明

- 评审发现的 2 条 should-fix 已修复：迟到回调栅栏（+`TestHandlerFencedAfterStop`）、NewRequest 解析失败路径 webhook token 脱敏（+两种失败阶段断言；期间确认 net/url 对数字端口解析期放行、拨号期才失败——测试按两阶段各设一例）。
- 首轮 executor 超时中断仅留下 go.mod/go.sum 骨架，重派后全量完成；无半成品混入。
- Stop×重拨最长 ~50s 互斥窗口与死耳静默重拨：已文档化/登记，见 summary 不做声明。
- 未 push。
