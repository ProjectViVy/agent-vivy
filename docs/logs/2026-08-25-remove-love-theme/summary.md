# 移除恋粉（love）主题

## 变更内容

用户反馈恋粉主题不好看，整主题移除。皮肤功能保持 4 套：
Vivy 蓝（默认）、简约粉白、深蓝夜色、Miku 青。

- `ui/src/hooks/use-theme.ts`：`THEME_IDS` 与 `THEMES` 移除 `love` 条目。
- `ui/src/styles.css`：删除 `[data-theme="love"]` 令牌块。
- `ui/index.html`：反闪烁内联样式与引导脚本的 `APPEARANCES` 表移除 love。
- `ui/src/hooks/use-theme.test.ts`：DOM 应用/持久化用例改用 `pink`；
  存储回退用例改用 `'love'` 作为非法值——显式覆盖"已删除主题的
  残留 localStorage 回退到默认"路径。

已存 `vivy.theme=love` 的浏览器：TS 侧 `readStoredTheme` 与 index.html
引导脚本的 id 白名单都会拒绝该值并回落 `default`，无需迁移。

## 明确不做

- 不动其余 4 套主题的任何取值。
- 不补新主题（如需替换位再议）。

## 验证

见 `verification.md`；验收路径见 `acceptance.md`。上一次交付记录：
`docs/logs/2026-08-25-vivy-ui-themes/`。
