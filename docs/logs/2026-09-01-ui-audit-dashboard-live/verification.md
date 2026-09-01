# Verification

## 门禁

- `just ci` — 通过（exit 0）：Go fmt/vet/test、headless 编译、plugin-ci 6
  module、UI `pnpm install --frozen-lockfile` + `typecheck`（tsc
  --noEmit）+ `vitest run` + `vite build` 全绿。demo-api.test.ts 两个
  dashboard 用例改写到 memory 面后全 suite 通过。
- `just ui-e2e` — 通过（exit 0）：10 passed / 1 skipped（cron 长链路
  spec 按既有基线跳过）。套件先 `pnpm build`（含本变更）再用 Playwright
  驱动真实浏览器打真实控制面（`runtime.spec.ts` 走真会话/Review/
  Settings）。

## Smoke 说明

- 本切片是用户可见变更；浏览器级证据 = `just ui-e2e` 全套（真实浏览器 +
  真实控制面 + 含本变更的构建产物），沿用 CH-C1-N3 先例（无组件专属
  spec 时以全套 e2e 为 smoke 替代）。
- 无 dashboard 专属 spec：overview 三格与具体数字没有断言级覆盖——
  `session/list` 与 `review/list` 在既有 spec 的应用层路径（会话列表/
  Review Center）已被真实后端 exercising；`background/list` 属同族
  只读 RPC。结论：接线正确性由类型 + 单测 + 全套 e2e 兜底，数字级
  断言留待后续补 dashboard spec（不阻塞本行——审查行的"接真实 RPC"
  目标已由代码事实达成）。
- RPC 传输为 WebSocket（`/rpc/bootstrap` → WS upgrade），无 curl 直呼
  路径；未做手工 curl smoke。
