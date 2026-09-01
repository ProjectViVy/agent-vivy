# Verification

| 命令 | 结果 |
|---|---|
| `pnpm typecheck`（ui/） | passed |
| `pnpm test -- --run`（ui/） | passed——24 files / 196 tests |
| `just ui-e2e`（pnpm build 重建嵌入 bundle + 全量 e2e，含新 lifecycle-readonly.spec.ts） | passed——13 passed + 1 skipped（runtime.spec 无 provider 跳过），含 `lifecycle-readonly.spec.ts:5 992ms` |
| `just ci`（fmt-check + vet + go test + headless + plugin-ci + UI：typecheck/test/build） | 见下 |

要点：
- 新 spec `lifecycle-readonly.spec.ts` provider 无关：设置 → Vivy 功能 →
  「打开生命周期」→ 当前 Species + 权威说明行可见；创建/启动评测/确认提升/拒绝
  按钮全为 0；Generations/Evals/Promotions tab 可切换且无写按钮。
- 只读列表空态即空列表（无 spec 断言具体条目，避免依赖后端状态）。
