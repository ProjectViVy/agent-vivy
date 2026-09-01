# Verification

## 门禁

- `just ci` — 通过（exit 0）：Go fmt/vet/test、headless 编译、plugin-ci
  6 module、UI install + `tsc --noEmit` + `vitest run` + `vite build`
  全绿（`runActive` 导出、卡片订阅、busyHint 键均过类型与 lint）。
- `just ui-e2e` — 通过（exit 0，10 passed / 1 skipped）：真实浏览器 +
  真实控制面；`compaction-setting.spec.ts` 只断言标签文案（不点按钮），
  按钮禁用逻辑不影响既有断言。

## Smoke 说明

- 忙碌态的行为级浏览器断言（发起 turn → 设置页按钮禁用 → 运行结束
  恢复）无组件专属 spec；按 CH-C1-N3 先例以全套 e2e 为 smoke 替代，
  行为路径在 acceptance.md 供人工复核。
- 409 兜底路径未动（后端与 `compactNow` catch 原样），竞态行为不回归。

## 复核证据（静态）

- `internal/runtime/compaction_service.go:123-127`：busy =
  `len(s.active) > 0 || len(s.pending) > 0`（引擎全局）→ 409。
- `ui/src/lib/store.ts`：`runActive` 谓词导出；`currentRun` 由订阅事件
  实时更新（run.started → active，run.completed/failed/cancelled → 终结）；
  `backgroundRuns` 由 init 与 `loadBackgroundRuns()` 维护。
- 卡片禁用条件 `runActive(currentRun) || backgroundRuns.some(runActive)`
  覆盖两个可观察源；跨端前台运行的盲区记录于 summary.md。
