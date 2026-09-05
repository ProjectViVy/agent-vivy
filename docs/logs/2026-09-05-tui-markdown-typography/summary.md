# TUI Markdown 排版（Crush 对齐）

## 已交付

- 全屏聊天的助手/用户气泡不再把 Markdown 当纯文本换行。`sdk/tui/view` 用 `github.com/charmbracelet/glamour` v1（Charm v1 / lipgloss v1，不是 `charm.land/glamour/v2`）按 Vivy 调色板渲染结构。
- 排版语言对齐 Crush 的视觉合同，不是源码移植：H1 药丸、H2+ 保留 `##` 前缀、列表 `•`、引用 `│ `、行内 code chip、围栏 chroma、链接分层、思考块 QuietMarkdown + `┊` 槽。
- 文档层去掉 Glamour 默认的大段空白；已完成消息按 session/width 缓存；流式气泡仍每帧全量渲染当前一条，光标 `▌` 加在渲染后的末行。
- 模型正文仍先剥 ANSI / 控制符 / bidi；glamour 失败则回退既有 `wrapText`。附件和 `@file` chip 仍只显示 metadata，不进 Markdown 解析。
- 内置 `vivy tui`、独立 `vivy-code`、packed `faces/tui` 共用这一条 render 路径。

## 边界

- 没有移植 Crush 的 streaming stable-prefix 缓存（`TUI-MD-STREAM-CACHE`）。
- 工具卡片仍是 compact wrap + unified diff 着色（`TUI-MD-TOOL-RESULTS`）。
- 没有 Charm v2、没有自定义 chroma formatter、没有主题切换器。
- 没有改 `surface.Message`、控制面或 Studio。
- 本次不是发布操作，因此没有 `release.md`。
