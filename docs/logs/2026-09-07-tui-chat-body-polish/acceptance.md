# 人类验收

启动任一真实 TUI 路径（`vivy-code.exe` 交互终端，或 `vivy tui`）：

1. **F5 工具卡展开**：让 agent 执行一个输出较长的工具（如列目录/读文件）。工具卡正文默认最多 8 行，末行显示 `… N more lines · ctrl+o expand`；按 `ctrl+o` 后同一卡片显示完整输出、标记消失；再按一次恢复 8 行。`tui.debug: true` 的用户不受影响（始终全量）。
2. **F9 reasoning 折叠**：在开思考的模型下提问。reasoning 段默认完整显示（┊ 竖线样式）；按 `ctrl+r` 后每段 reasoning 变为一行 `reasoning · N 行 · ctrl+r 展开`，正文不再可见；再按一次还原。
3. **F13 空会话 hero**：`ctrl+n` 新建会话（或首次启动）。聊天区出现 `Vivy™ VIVY CODE` 标识、「寻找真心之旅」、当前工作目录（如有）与键位提示；输入第一条消息后 hero 消失。
4. **快捷键面板**：`ctrl+x` 打开面板，能看到 `ctrl+o 工具输出` 与 `ctrl+r reasoning` 两行。
5. **不干扰**：审批/提问 gate 弹出时按 `ctrl+o`/`ctrl+r` 不改变聊天体状态；`ctrl+s` 会话对话框内 `ctrl+r` 仍是重命名。

以上 1–3 均为纯展示行为，Journal 数据与发给模型的上下文不变。
