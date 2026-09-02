# acceptance — FACE-TUI-1 F3

## 人类如何确认「出厂 TUI 脸」真的在了

1. **打包一张 TUI 嘴**（在仓库根）：
   ```text
   vivy-sdk pack --face tui
   ```
   输出 generation（如 `gen_d6fccc14e3958687`），manifest 里 `recipe.face: "tui"`、`face.kind: "tui"`、grants `tty/argv/rpc.client`。
2. **管道场景响亮失败**（不接终端跑这张脸）：
   ```text
   vivy.exe run "hi" > out.txt 2> err.txt
   ```
   `err.txt` 应为 `tui: this face needs an interactive terminal (stdout is not a tty); ...`，退出码 1——脸拒绝在非终端里装死。
3. **终端里真过一轮**（成功标准，需真 TTY，如 Windows Terminal）：
   ```text
   vivy.exe run "帮我改一下 README 里的错字"
   ```
   预期：全屏 TUI（侧栏会话列表 + 聊天区 + 编辑器）；prompt 自动开首轮并流式渲染；工具卡出现；审批挂起时弹出 overlay，按 `y`/`n` 应答；提问 overlay 输入回车应答；`esc` 取消运行；`tab`/方向键切换会话；`^n` 新会话；`q`/`^c` 退出。
4. **同一 Journal 回放**：随后启动网页世代二进制（`just run` + `http://127.0.0.1:3015`），会话列表里能看到同一会话与刚才那轮对话、工具卡与审批结果——两具身体读同一 Journal（不要求同时运行）。
5. **网关不含这张脸**：仓库原样 `just build`（或 `just ci`）出的默认 vivy.exe 不引入 TUI 器官——`vivy run` 在无 face 注册的 committed body 里走 headless 路径；只有 `pack --face tui` 的制品带这张嘴。

## 边界

- 设置页、审阅中心全量、多脸同居、驻留网关探路客户端（`vivy tui`）均不在 F3 范围（见 summary.md「明确未做」）。
