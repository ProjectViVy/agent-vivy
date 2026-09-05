# TUI 模式循环、强度与彩色 chrome

## 已交付

- 空输入 `shift+tab` 循环 智能 → 计划 → 只读。智能/只读会改 permission（smart/cautious）；计划走既有 `turn/start.mode=plan`。trusted 仍在 Ctrl+Y。
- 模型名显示思考强度：`on` → `(high)`（金色），`auto` → `(auto)`。不新增 effort RPC。
- 输入框下左侧：`model(high) · provider  42%`；百分比按占用着色（绿 / 金 / 玫瑰）。未知窗口不编造上限。
- 右侧：`shift+tab 智能` 与帮助/快捷。
- 全屏 TUI 默认 TrueColor；仅 `NO_COLOR` 才降级。用户消息与左槽、右栏 host 对比加强。

## 边界

- 忙碌或正在输入时 `shift+tab` 不切模式。
- 命令面板打开时 `shift+tab` 仍是上一行。
- 没有做 high/medium/low 的新内核旋钮。
