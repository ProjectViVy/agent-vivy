# CH-C7c — verification

日期：2026-08-30。工作区：worktree `agent-vivy-channel-c2`（顺序复用），分支 `feat/channel-c7c`，基线 d5f4506（含 C1–C7b）。

## GOAL 运行方式（子代理分工）

- executor（写盘）：plugins/discord 全套 + pion 封禁 + 夹具；评审 note（LogLevel 显式钉死）补修。
- reviewer（只读独立评审）：**PASS**；discordgo v0.29 源码五项主张逐条核实全真（Open 同步到 READY / reconnect 无视 Close / Close 发合成 DISCONNECT / 具名处理函数类型不被识别 / ChannelMessageSendComplex 纯 REST）。
- GOAL 持有人：pack 候选证据 + 落地。

## 命令与结果

| 命令 | 结果 |
|---|---|
| `gofmt -l`（全树含 plugins/discord 与夹具） | 干净 |
| 根 `go build ./...` / `go vet ./...` / `go test ./...` | 全 ok |
| `cd plugins/discord && go vet && go test ./... -count=1` + `go test -race` | ok（0.3s / 1.4s） |
| `go run ./sdk verify plugins/discord` | ok |
| `go run ./sdk verify sdk/internal/testdata/bad-pion-import` | **exit 1 拒绝**：「import of github.com/pion/webrtc/v3 is forbidden: pion/webrtc is banned in plugins (no voice in Vivy channels)」 |
| `go test ./sdk/... -count=1`（新夹具 + 既有全部） | ok |
| `go run ./sdk pack --with discord --out <tmp>` + inspect-artifact + `go version -m` | 候选 `gen_4d64767f37cb958e` 链接 discordgo v0.29.0；EXE 内 pion 模块 **0** |
| `go list -deps ./cmd/vivy \| grep -c discordgo` | 0 |
| `git diff d5f4506 -- go.mod go.sum` | 空 |
| 插件树 pion grep | 0（生产代码零引用，仅声明缺席的注释） |
| `just ci` | **exit 0**（Go 全部包 ok + UI 21 文件 / 172 测试 + vite build） |

## 验收清单（CH-C7c.md §7）

- 候选 DM/文本频道文本（normalize + 派发 + Send 测试矩阵，回环/fake 全覆盖）✅
- 依赖图无 pion（物种与候选双验证）✅
- 空 allow_from 拒绝（Host 回归绿）✅
- voice.go / pion / TTS / slash 零移植，且 pion 封禁进 verify 全 seam 生效 ✅

## 诚实声明

- RESUME 续传缺失（v0.29 未导出 session id/seq）：重拨间隙事件有界丢失——防卡死优先，有意偏离，README 记录。
- 重启后旧 session 已派发回调落进新生命周期的理论窗口（与姊妹插件同型，有界）。
- `just ci` 不覆盖 plugins/*（结构性缺口，登记 CH-C7c-N1，建议 plugin-ci 配方）。
- inspect 封禁理由串对非 webrtc 的 pion 模块也打 "pion/webrtc" 字样（装饰性；测试钉住该子串）。
- 真实 Discord 冒烟未做（无凭据 + Intent 需开发者面板开通）。
- 未 push。
