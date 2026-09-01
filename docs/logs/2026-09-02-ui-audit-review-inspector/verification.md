# Verification

| 命令 | 结果 |
|---|---|
| `pnpm typecheck`（ui/） | passed |
| `just ui-e2e`（pnpm build 重建嵌入 bundle + 全量 e2e，含新 run-inspector-review.spec.ts） | passed——12 passed + 1 skipped（runtime.spec 无 provider 跳过），含 `run-inspector-review.spec.ts:6 789ms`、`approvals-nav.spec.ts 698ms` |
| `just ci`（fmt-check + vet + go test + headless + plugin-ci + UI：typecheck/test/build） | passed（exit 0） |

要点：
- 新 spec `run-inspector-review.spec.ts` provider 无关：设置 → Vivy 功能 →
  RunInspector「审批」tab 可见可点、空态文案正确、空态下无批准/拒绝按钮。
- 内联决策真实路径（审批出现 → inspector 卡上批准/拒绝 → 队列同步消失）需要
  provider 触发工具审批，本环境不可达，已在 summary 记为 not done。
