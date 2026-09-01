# Verification

## 门禁

- `just ci` — 通过（exit 0）：Go fmt/vet/test、headless 编译、plugin-ci
  6 module、UI install + `tsc --noEmit` + `vitest run` + `vite build`
  全绿（`reviewBusyIds: string[]` 改型、respondReview 按 id 单飞、
  ApprovalsView/_layout 引用点全部过类型与 lint）。
- `just ui-e2e` — 通过（10 passed / 1 skipped）：真实浏览器 + 真实控制面；
  `runtime.spec.ts` 覆盖 review 主路径（打开 Review sheet + 响应流），
  busy 锁改型未回归既有断言。

## Smoke 说明

- 双记录并发响应的浏览器级断言（响应 A 期间选 B、B 解锁后可响应）
  无组件专属 spec；按 CH-C1-N3 先例以全套 e2e 为 smoke 替代，人工观察
  路径在 acceptance.md。
- WebSocket RPC 传输（`/rpc/bootstrap` → WS upgrade）无 curl smoke 路径。

## 复核证据（静态）

- `ui/src/lib/store.ts` `respondReview`：`includes(id)` 单飞（同 id
  重复响应 early-return），busy 集合按 id 增删（finally 过滤）；乐观
  状态映射按 id，`loadReviews()` 幂等 GET，并发安全。
- 全库无 `reviewBusyId`（单数）残留（grep 复核）；cron/mcp 等组件的
  同名局部变量不受影响。
- `ui/src/routes/_layout.tsx`：sheet 关闭守卫 `reviewBusyIds.length > 0`
  ——任一响应在途时不可关（与原 `!!reviewBusyId` 语义一致）。
