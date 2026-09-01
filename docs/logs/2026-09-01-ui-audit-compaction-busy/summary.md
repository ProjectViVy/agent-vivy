# UI-AUDIT-COMPACTION-BUSY — 立即压缩的忙碌预判

## 问题（审查行）

Settings → 上下文压缩卡的「立即压缩」可在引擎忙碌时点击，用户点了才收到
409 `ErrCompactionBusy`。后端 busy 是**引擎全局**的：
`compaction_service.go` 的 `CompactSession` 在 `len(s.active) > 0 ||
len(s.pending) > 0` 时拒绝——任一会话的活动/排队运行都挡住手动压缩
（运行本身已在运行内压缩）。

## 修复（UI 预判 + 409 兜底）

`CompactionSettingsCard` 现订阅 store 的运行真相：

- `runActive(currentRun)` —— 当前挂接会话的在途 turn（订阅事件实时更新）
- `backgroundRuns.some(runActive)` —— 后台注册表中的非终结态运行
  （`completed/failed/cancelled` 之外）

任一命中即禁用「立即压缩」并显示 amber 提示
`settings.compaction.busyHint`（en/zh）："有运行进行中，压缩会在运行内
自动进行；请等运行结束。" 「刷新占用」按钮现在同时调
`loadBackgroundRuns()` 重取后台注册表，用户可手动复核忙碌状态。

`store.runActive`（终结态谓词，单一判断源）由模块私有改为导出复用。

## 残余竞态（记录，不修）

- 预判与点击之间新 run 可能启动——409 保留为兜底，`compactNow` 的
  catch 已把错误消息显示在 feedback 区。
- 其他客户端（如另一浏览器标签）挂接的**前台**运行对本 tab 的 store
  不可见（未进后台注册表）；此类跨端忙碌只能由 409 兜底。
- `store.ts` 与 `DashboardView.tsx` 各有一份终结态集合字面量；卡片已
  收敛到 `runActive`，Dashboard 的计数谓词形态不同（filter 计数），
  暂不强行合一。

## 变更清单

- `ui/src/lib/store.ts`：`runActive` 加 `export`。
- `ui/src/components/settings/CompactionSettingsCard.tsx`：忙碌订阅 +
  按钮禁用 + busyHint + 刷新联动 `loadBackgroundRuns`；doc comment 更新。
- `ui/src/i18n/en.ts` / `zh.ts`：`settings.compaction.busyHint`。
