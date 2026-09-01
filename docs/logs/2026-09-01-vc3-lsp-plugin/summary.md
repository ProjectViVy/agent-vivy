# 2026-09-01 — VC-3 切片 1：proc.spawn 能力 + LSP 插件（lsp_diagnostics）

## What changed

本切片是 VC-3（D4：LSP = vivy-sdk 独立 module 插件，tool_world seam）的第一个
可交付切片，两部分一体交付（能力 + 消费者，缺一不可）：

### 内核侧：proc.spawn 能力（kernel/SDK）

- `sdk/plugin/plugin.go` — 新 grant `proc.spawn`；`SpawnSpec`/`Proc` 类型；
  `plugin.Env` 增加第五个方法 `Spawn(ctx, spec) (Proc, error)`。`os/exec` 在
  插件源码中保持禁封——spawn 是 kernel-hosted 能力，与 Listen 属 ChannelHost
  同构。子进程 cwd 固定在插件 workspace，outlives 工具调用（插件持有关照
  责任直至 Close）。
- `sdk/internal/manifest.go` — proc.spawn 仅 tool_world seam 可声明（tool seam
  声明即 verify 失败），与 channel 族 grant 的 seam 限制同型。
- `sdk/internal/testdata/bad-procspawn-seam/` + `verify_test.go` — 负面 fixture
  与断言。
- `internal/pluginhost/host.go` — `hostedEnv.Spawn` 实现：grant fail-closed；
  命令 = 裸 PATH 名或 workspace 相对路径（绝对路径/逃逸拒绝）；`exec` 用
  `context.WithoutCancel`（保住 context 值、丢弃 run 取消，语言服务器必须
  跨调用存活）；stdin/stdout/stderr 三管道；`Close()` = kill + reap。
- `internal/pluginhost/host_test.go` — 真实子进程测试（echo/cd/pwd 证明管道
  与 workspace cwd 固定；无 grant 拒绝；逃逸拒绝；Close 杀子进程）。

### 插件侧：plugins/lsp（独立 module，D4 首例）

- `plugins/lsp/` 独立 module（`module example.com/vivy/plugins/lsp` +
  `replace agent-vivy => ../..`），零第三方依赖（含 go.sum 无新增）。
- `jsonrpc.go` — 手写 LSP base protocol 编解码（Content-Length 帧、
  Content-Type 跳过）。研究行原文评估 powernap（MIT）fallback 自写约 1-2k
  行；实际自写仅 ~110 行即覆盖本插件所需子集，故未引入 powernap——
  供应链零新增。
- `protocol.go` / `languages.go` / `client.go` / `manager.go` / `plugin.go` —
  LSP 客户端（initialize/initialized、didOpen/didChange 全文同步、
  publishDiagnostics 捕获与"新发布"等待语义）；manager 按 (语言, workspace
  root) 键控懒启动 + 死进程替换 + 空闲收割（10 分钟，1 分钟巡检——插件契约
  无 Stop 钩子，收割即关闭故事）；工具 `lsp_diagnostics`（effect read，
  仅保存内容：env.OpenRead 读盘 → didOpen/didChange → 等新鲜发布 → 格式化
  `path:line:col: severity: message [source]`，wait_ms 默认 3000 上限
  15000，超时返回现有结果并附注）。
- `vivy-plugin.json` — seam tool-world，grants [fs.read, proc.spawn]，
  tools [lsp_diagnostics]。
- `plugin_test.go` — 无真实服务器依赖的确定性端到端：fakeEnv.Spawn 起
  内存假语言服务器（io.Pipe + 真 jsonrpc 帧协议），覆盖 initialize →
  didOpen → publish → 格式化全链路 + 第二次调用复用连接（didChange 路径）
  + 参数校验不触发 spawn。

## 验证命令

见 `verification.md`。

## Explicitly not done（本切片不做）

- **编辑后诊断回填**（write/patch/multiedit 结果附加 LSP 诊断）——需内核
  write 路径与插件诊断的衔接设计（VC-3 行内为实现设计点），下一切片。
- **lsp_definition / lsp_references / lsp_symbols / lsp_rename** 工具族
  后续分批；rename 走 write 审批。
- **文件版本 history**（L2 会话级回退）——RB-1 行挂靠，待用户拍板 O1..O6。
- **UI 文件预览 / 语法高亮 / read_file 图片**。
- **powernap 评估**——自写客户端已覆盖，评估项作废（供应链零新增）。
- 诊断位置为 LSP UTF-16 code unit 口径（协议默认），未做编码协商
  （positionEncoding）与 utf-8 换算——gopls 场景行号一致，列号可能偏差，
  后续切片再议。
- 诊断仅取工具调用目标 URI 的发布（server 可能推其他文件，暂不展示）。

## Crush 对齐口径

Crush 为 FSL-1.1-MIT：本切片 = 行为/协议对齐（LSP 诊断作为编码质量反馈），
零代码拷贝。`lsp_diagnostics` 是 Crush 已有行为（LSP 诊断 + lint/type 错误
直达模型）；本切片未添加 Crush 没有的功能面。诊断回填（Crush 的关键机制）
属下一切片。
