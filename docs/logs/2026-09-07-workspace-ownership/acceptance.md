# Acceptance — TUI-WORKSPACE-OWNERSHIP

## 一个人如何确认它生效

1. 启动后端与 UI（`just dev`），打开 `http://127.0.0.1:3015`，发起一次带工具的会话，
   让 run 产生 workspace 后打开文件面板 —— 文件列表与预览行为不变（回归面）。
2. 对同一后端直接发一条未知 run id 的 RPC（例如经浏览器控制台调用
   `workspace/list`，`run_id: "run_i_do_not_exist"`）：应得到
   `run not found` 404，且 `data/` 下 workspace 根目录**不出现新目录**。
   （修复前：返回成功空列表并在磁盘创建 `run_i_do_not_exist/`。）
3. 对一个 Journal 里存在、但从未启动过的 run id 调 `workspace/list`：应得到
   `run workspace not found`，同样零目录创建。
4. TUI `vivy-code` 的 `/files <run>` 命令对未知 run 显示错误结果，不再静默成功。

## 回归风险

- run 正常产生 workspace 后，文件面板行为不变（既有 runtime/rpc 测试全绿）。
- run 刚创建、workspace 未生成时，面板从"空列表"变为显示
  `run workspace not found` 错误文本 —— 语义更诚实，属预期变化。
