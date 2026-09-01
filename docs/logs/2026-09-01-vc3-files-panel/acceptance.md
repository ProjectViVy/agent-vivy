# Acceptance — VC-3g files panel

## 人怎么看它工作了

1. `just dev`（或 split pair）打开 `http://127.0.0.1:3015`。
2. 头部右侧出现一个文件夹图标按钮（Files，与 Todos/Review Center 并列）。
3. 发起一次会写文件的 run（例如让 vivy 写一个 `notes.md`），run 进行中或结束后
   点击 Files：
   - 面板列出该 run 工作区里的文件（相对路径 + 大小，按路径排序）；
   - 点击文本文件，右侧/下方出现带语法高亮的预览（深色 github 主题）；
   - 超大文件显示截断标记；二进制文件显示"二进制文件"占位而非乱码；
   - 文件多于 2000 个时列表显示截断标记（正常 run 不会触达）。
4. 没有 run 时不误报：面板显示"开始一次运行后可查看其工作区文件"空态。
5. 中文界面显示"文件"/中文文案，英文界面显示 Files/English 文案。

## 边界（不该发生的事）

- 输入 `../`、绝对路径、盘符路径的读取尝试全部被拒绝（真实服务器冒烟验证，
  错误不泄露内部细节）。
- 不存在任何写/删/下载入口——面板是纯只读预览；恢复/回退归文件版本 history
  （等 O1..O6 裁决）。
- WorkspaceFiles 未装配的部署（如 workspace 关闭）下，`workspace/*` RPC 返回
  MethodNotFound，UI 不假装有空列表。

## 无 provider 机器的验收口径

本机无 OPENAI/ANTHROPIC key：浏览器验收只覆盖面板壳态（空态 + 开关），
"run 产生文件 → 面板高亮预览"的全流程在有 key 机器上走一遍即完全验收；
内核 list/read 全链路已由真实服务器 WebSocket 冒烟覆盖（见 verification.md 表格）。
