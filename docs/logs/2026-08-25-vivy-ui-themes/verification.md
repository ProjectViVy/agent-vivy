# 验证记录 — Vivy UI 皮肤功能

## just ci（仓库根目录）

命令：`just ci`（= fmt-check + vet + go test + headless-compile + ui-ci）

结果：**通过（exit 0）**，2026-08-25。

- go: fmt-check / vet / `go test ./...` / `go test -run '^$' -tags vivy_headless`
  全绿。
- ui-ci：`pnpm install --frozen-lockfile` + `pnpm typecheck`（tsc 无错）
  + `pnpm test`（8 个测试文件 30 例全过，含新增
  `src/hooks/use-theme.test.ts` 8 例）+ `pnpm build`（✓ built in 3.11s）。

## 浏览器实走（smoke-for-user-visible-change）

环境：复用已在运行的 split pair（Vite `http://127.0.0.1:3015` + 控制面
8787，浏览器为 ZCode 内置浏览器，Vite HMR 加载本次改动）。

路径与结果：

1. 打开 `http://127.0.0.1:3015/`，进入"设置"→"通用"tab：
   - 新"主题"卡渲染在"应用信息"之后，5 个主题按钮齐全，
     "Vivy 蓝"初始 `[pressed]` 且带"已选中"图标。
   - 迁移预览中旧的假主题卡已消失（剩余：聊天显示 / 缓存 / 关于）。
   - `DemoNote` 只覆盖迁移预览区域。
2. 点击"恋粉"：
   - `<html data-theme="love">`，`html.dark` 计数 0。
   - 截图确认：浅粉背景/侧栏、白色卡片、粉色选中态，无渲染错误
     （截图：`vivy-theme-love.png`，临时目录）。
3. 点击"Miku 青"：
   - `<html data-theme="miku">`，`html.dark` 计数 1（`dark:` 变体生效）。
   - 截图确认：深色背景/侧栏/卡片 + 青色强调，文字对比正常
     （截图：`vivy-theme-miku.png`）。
4. 刷新页面（持久化 + 反闪烁引导）：
   - 刷新后 `<html data-theme="miku">`、`.dark` 保持，页面仍为 Miku
     深色，证明 `vivy.theme` localStorage 与 index.html 引导脚本工作
     （截图：`vivy-theme-miku-reload.png`）。
5. 切回"Vivy 蓝"：`data-theme="default"`、`.dark` 移除，恢复默认。

已知说明：本会话自行启动的 `just run` 因缺 `OPENAI_API_KEY` 退出、
`pnpm dev` 因 3015 已被占用退出——两者均有同端口的既有服务在跑，
冒烟使用既有 pair 完成，不影响结论。

## 未验证项

- 真实 Provider 下的对话流（本次改动不触及 RPC/消息链路）。
- 嵌入式 UI（:8787）未单独实走：它打包同一 `index.html` + `styles.css`，
  主题代码路径与 Vite 完全一致，`just ci` 的 `pnpm build` 已覆盖构建。

## 补记（2026-08-25，提交前复查）

本文所述 5 主题中的"恋粉"（`love`）随后被整体移除，皮肤收敛为
4 套；`use-theme.test.ts` 同步改用 `pink`。该迭代及其验证见
`docs/logs/2026-08-25-remove-love-theme/`（移除后仓库根 `just ci`
全绿，exit 0）。

