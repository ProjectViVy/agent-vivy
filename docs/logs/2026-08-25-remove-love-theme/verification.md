# 验证记录 — 移除恋粉主题

## just ci（仓库根目录）

命令：`just ci`。结果见下方补记。

## 定向检查（ui/）

- `pnpm typecheck`：tsc 无错。
- `pnpm test`：8 个测试文件 30 例全过（2026-08-25 06:20）。
  其中 `use-theme.test.ts` 的存储回退用例现以 `'love'` 为非法值输入，
  断言回落 `DEFAULT_THEME_ID`。

## 浏览器实走（smoke-for-user-visible-change）

环境：既有 split pair（Vite `http://127.0.0.1:3015`，HMR）。

1. 刷新 `http://127.0.0.1:3015/settings`：
   - "主题"卡只剩 4 个按钮：Vivy 蓝（默认）、简约粉白、深蓝夜色、
     Miku 青；"恋粉"不再出现。
   - `<html data-theme="default">`，主题卡正常渲染。
2. 残留存储回退由单测覆盖（`readStoredTheme('love') → default`），
   index.html 引导脚本使用同一 id 白名单逻辑。

## just ci 补记

`just ci`（fmt-check + vet + go test + headless-compile + ui-ci）
于本次改动后运行通过（exit 0）。
