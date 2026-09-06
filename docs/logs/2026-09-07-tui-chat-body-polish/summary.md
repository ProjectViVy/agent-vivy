# Lane C 聊天体：F5 工具卡 ctrl+o 展开 · F9 reasoning ctrl+r 折叠 · F13 空会话 hero

## 已交付

全部改动位于共享呈现层 `sdk/tui/view`，`vivy-code.exe`、`vivy tui`（含 `--live`）与打包 TUI face 三条路径同步生效。

- **F5 工具卡 ctrl+o 展开**：新增会话内 `toolExpanded` 开关（`KeyCtrlO`，gate 存在时不响应），生效条件为 `tui.debug` 配置或 ctrl+o 临时展开；`compactToolLines` 省略标记改为 `… N more lines · ctrl+o expand`。工具卡渲染不进 mdCache，无缓存失效问题；`tui.debug` 配置语义不变。
- **F9 reasoning ctrl+r 折叠**：新增会话内 `reasoningCollapsed` 开关（`KeyCtrlR`，gate 存在时不响应）；折叠时 reasoning 消息渲染为单行摘要 `reasoning · N 行 · ctrl+r 展开`（保留 ReasoningBar/Reasoning 样式与 chips 逻辑），展开为原状。`messageMarkdownKey` 与缓存 `ensure` 增加 `collapsed` 维度，保证同宽度下切换不读旧渲染。
- **F13 空会话 hero**：`chatLines` 空分支由 3 行占位升级为 hero（`Vivy™ VIVY CODE` wordmark、「寻找真心之旅」、有值时的 CWD 行、命令与键位提示），窄宽度走既有 `truncate`；非空会话不渲染。
- 快捷键面板（ctrl+x）新增 `ctrl+o 工具输出`、`ctrl+r reasoning` 两行。

## 明确未做

- 未改 surface/domain 契约、驱动层、Journal、provider 或 Eino 编排（纯展示层，无 Eino surface 适用）。
- 未做逐卡定点展开/折叠（ctrl+o/ctrl+r 为全会话级开关）；未持久化开关状态（每次启动回到默认）。
- 未处理超窄宽度下 hero wordmark 的硬截（padBlock 兜底，化妆级，未在本轮范围内）。
- 仓库根的两个未跟踪 scratch（`sdk/tui/view/zpreview_test.go`、`tui-composer-shot.png`）属于前 lane，未纳入本交付。

## 过程

独立 worktree `feat/tui-chat-body-polish`（VC 轨道并行隔离规则）。子 agent 分工：builder 实现 F5/F9/F13 与测试，reviewer diff 审查（SHIP，3 P2：键位标签改中性措辞、测试去除未导出字段断言，均已修；hero 窄宽截断记为未做）。监督者负责集成、测试与 `just ci`。
