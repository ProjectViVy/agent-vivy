# Verification

## 门禁

- `just ci` — 通过（exit 0）：Go fmt/vet/test、headless 编译、plugin-ci
  6 module、UI install + `tsc --noEmit` + `vitest run` + `vite build`
  全绿（`openSeq` 状态、aria-expanded 展开、payload pre 均过类型与 lint）。
- `just ui-e2e` — 通过（exit 0，10 passed / 1 skipped，26.3s）：真实浏览器
  + 真实控制面；`runtime.spec.ts` 覆盖聊天页主路径（含 run 事件流渲染），
  事件行 DOM 改动（button 包裹 + 可展开 pre）未回归既有断言。

## Smoke 说明

- Inspector 事件展开/收起交互无组件专属 spec；按 CH-C1-N3 先例以全套
  e2e 为 smoke 替代（Run Inspector 位于聊天页，runtime spec 构建自含
  本改动的源码），展开行为在 acceptance.md 供人工复核。
- WebSocket RPC 传输（`/rpc/bootstrap` → WS upgrade）无 curl smoke 路径。

## 复核证据（静态）

- `ui/src/components/chat/RunInspector.tsx`：事件行由 `title={JSON.stringify(...)}`
  改为 `<button aria-expanded>`（键盘可达），展开态渲染
  `JSON.stringify(event.payload ?? null, null, 2)` 进限高 pre
  （max-h-48 滚动），`openSeq` 逐行独立切换。
- 零新增 i18n 键：payload 为结构化数据，直接 JSON 打印，无文案。
