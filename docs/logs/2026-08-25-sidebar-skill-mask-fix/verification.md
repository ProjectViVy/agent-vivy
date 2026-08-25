# 验证

## 自动化

- 仓库根 `just ci`：**exit 0**，全阶段通过（fmt-check / `go vet` / `go test ./...`
  全部 ok / headless-compile / `ui-ci`：`pnpm typecheck` + `vitest run`
  **40/40 通过** + `vite build` 成功）。
- i18n 一致性：`nav.evolution` / `nav.evolutionPending` / `nav.evolutionUnavailable`
  同时加进 zh/en，`ui/src/i18n/index.test.ts` 的 zh/en 叶子键一致性断言通过。

## 浏览器实路（http://127.0.0.1:3015，split Vite）

桌面（1280×720）与移动端抽屉（390×844）均验证：

1. 侧边栏 Vivy 分组出现 `button "进化 待实现"`：图标 + 文案 + 「待实现」Badge；
   它是 `<button>` 而非 `<Link>`（DOM 快照无 `/url`）。
2. 点击「进化 待实现」：出现提示「进化功能暂未接入」，URL 保持在当前页
   （`/skills`）不跳转。
3. `/skills` 下高亮检查：仅「Skill」`<a>` 带 `active` 类；「进化」按钮无高亮
   样式；其余导航项均无 active。双重高亮不再出现。
4. 逐一点击其他导航项（聊天 / 中控台 / 定时任务 / 人格 / 面具 / 记忆 / 记事本 /
   MCP），每次用 `dom_cua.get_visible_dom()` 检查 Skill 项 bounding rect 均无
   元素重叠。
