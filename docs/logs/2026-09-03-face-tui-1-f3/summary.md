# summary — FACE-TUI-1 F3（出厂 faces/tui 器官）

日期：2026-09-03。FACE-TUI-1 轨道收官片：出厂交互式薄 TUI 器官 `faces/tui`（VIVY-FACE-PACK.md F3），F0/F1/F2/§14 之上补齐最后一张脸。docs 状态见 `docs/TODO.md` FACE-TUI-1 行（已翻 DONE）。

## 交付内容

- **faces/tui 独立 module**（`example.com/vivy/faces/tui`，go.mod require bubbletea v1.3.10 / lipgloss v1.1.0 与根 module 同版本 + `replace agent-vivy => ../..`；§14①：TUI 依赖永不进 gateway 世代的必经 import——committed body 的 face 注册器仍是 nil，器官只在 `pack --face tui` 时经 build-time overlay 进入制品）。
- **manifest**：`vivy-plugin.json` — `apiVersion vivy.plugin/v0`、`seam: face`、`face{kind: tui, listen: false}`、grants `tty/argv/rpc.client`（F2 信封校验直接生效）。
- **代码布局**（对照 kernel `internal/tui` 的薄拷贝，verifier 对所有 seam 禁 `agent-vivy/internal/` import，故入 module 而非引用）：
  - `surface/`、`view/`（Crush 式全屏外壳：侧栏会话列表 + 聊天投影 + 审批/提问 overlay + 编辑器）——从 kernel 逐字拷贝，仅做依赖本地化（demo 驱动移除 → `noDriver` 空壳兜底；domain 常量 → 包内本地常量；占位标题统一 VIVY）。
  - `events.go`：run 事件 → UI notice 解释器（delta/tool 卡片/approval gate/question gate/终态），事件词汇表以本地常量锁住 wire 字符串。
  - `live.go`：`Live` 驱动（surface.Driver 实现）——boot（session/list 空则 create）/turn 流式/审批应答/提问应答/取消/会话切换；**新增 InitialPrompt + ContinueNewest 语义**：`vivy run "prompt"` 打开 TUI 自动开首轮——`--continue` 附着最新会话，否则新建会话（标题照 headless 规则 60 rune 截断），applyBoot 在锁外回调 Send（修复自死锁）。
  - `rpc.go`：`client` 适配器把 `plugin.FaceEnv` 适配成 Call + OnNotify 方法面；八个控制面 RPC（session/create|list|messages、turn/start[带 `face: "tui"`]、run/subscribe、run/cancel、approval/respond、question/respond）。
  - `face.go`：seam-face 构造器 `New(plugin.FaceOptions) plugin.Face`；`Run`：TTY 检查（stdout 非 char device → 响亮失败，管道场景请用 headless 脸）→ initialize → program（AltScreen，输出走 `opts.Out`）→ 退出时若 run 仍在飞则 `run/cancel` + 轮询 `run/get` 到终态（Journal 落终点，不留悬 run）→ `FaceResult{Status}` 映射。
- **测试**（`face_test.go`，11 用例，plain + `-race` 绿）：fakeEnv 脚本化控制面——boot 建/取会话、turn 流式（断言 turn/start 的 `face: "tui"`）、approval gate 应答 + 他人 run 事件过滤、question 应答、会话切换载史、prompt 新会话/continue 附着、shutdown 取消悬 run、非 TTY 拒绝（拒绝发生在拨号前）、kind/视图外壳渲染冒烟。

## 成功标准（对照 VIVY-FACE-PACK.md F3）

- 「终端里过完一轮带审批的对话，同一 Journal 在网页世代的二进制里能回放」：控制面交互全部走 F1/F2 的进程内 JSON-RPC（同一 Journal），审批/提问 overlay 为一等交互（y/n 键 + 问题输入回车）；真实交互会话的人类验收路径见 acceptance.md（无 TTY 自动化环境，交互全流程留给人类确认；驱动层行为已被上述测试钉死）。
- 「网关二进制仍可不含这张脸」：faces/tui 独立 module + committed body nil 注册器，`just ci` 的 headless-compile 无 TUI 依赖即通过。

## 明确未做

- 设置页/审阅中心全量复刻（F3 合同明确薄刀）；`vivy tui` 驻留网关探路客户端未动；faces/web；listen:true；多脸同居（§14③）。
- TTY 自动化冒烟（真实交互键盘流）——见 acceptance.md 人类路径。
