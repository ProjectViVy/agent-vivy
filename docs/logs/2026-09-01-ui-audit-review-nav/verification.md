# Verification

| 命令 | 结果 |
|---|---|
| `pnpm exec playwright test e2e/runtime.spec.ts`（未重建 bundle 的初步跑） | passed——但 runtime.spec 的尾部断言位于 `hasRealProvider` 早退之后，本环境无 provider 实际未执行新断言；已改为独立 provider 无关 spec |
| `pnpm exec playwright test e2e/approvals-nav.spec.ts`（未重建 bundle 的第一次直接跑） | failed——欢迎向导异步渲染晚于一次性 `isVisible()` 检查，跳过动作被跳过、向导拦截导航点击（60s timeout）。修复：改为 5s 有界 `waitFor` 再跳过 |
| `just ui-e2e`（pnpm build 重建嵌入 bundle + 全量 e2e） | passed——11 passed（含 `approvals-nav.spec.ts:6 审批中心从主导航直达 662ms`），0 failed |
| `just ci`（fmt-check + vet + go test + headless + plugin-ci + UI：typecheck/test/build） | passed（exit 0） |

要点：
- 首次把导航断言塞进 runtime.spec.ts 是无效位置（provider 早退返回），改为 `approvals-nav.spec.ts` 独立 spec，无 key 环境同样执行；playwright webServer 服务的是构建后的嵌入 UI，必须先 `pnpm build`（`just ui-e2e` 已含）。
- 欢迎向导是异步弹出的，`isVisible()` 单次检查会竞态；需有界 `waitFor` 再决定是否跳过。
