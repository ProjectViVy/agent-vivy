# 验证命令与结果

| 命令 | 结果 |
| --- | --- |
| `cd ui; pnpm exec tsc -b --force` | 通过（zh/en 键位对等迁移后类型检查绿） |
| `just ui-e2e` | `1 skipped / 10 passed`（含新增 `compaction-setting.spec.ts`），退出码 0 |
| `just ci` | 退出码 0（golangci-lint + gofmt + go test ./... + ui tsc/eslint/vitest/build） |

## 诊断链（供复核）

1. 首轮 e2e 失败快照（`ui/test-results/compaction-setting-.../error-context.md`）
   显示卡内全部为 `settings.compaction.*` 原始键——直接证伪「键缺失」，因为
   字典里明明有同名块 → 定位为键位错配（`diva.compaction` vs `settings.compaction`）。
2. `grep -n "^  \},\|^  [a-z]" zh.ts` 勘定顶层结构：`settings:` 只覆盖 817–885，
   原 compaction 块在 `diva` 段；第一次迁移误落 `demo:` 段（第二次 e2e 仍原始键），
   第二次迁移才落进 `settings:` 段。
3. 第三轮 e2e 失败点移到英文字符残留断言：`DivaSettingsPreview` 的硬编码中文
   迁移说明段落含子串「最大 tokens」，非 exact 匹配误中 → 断言改 exact 并把
   预览区欠账拆出为独立 TODO 行。
4. 终轮 `just ui-e2e` 10 passed / 1 skipped；`just ci` 绿。

## 教训（写入本迭代）

- `zh.ts`/`en.ts` 顶层段落多且缩进相同，跨段搬移键块必须先用结构 grep
  勘定段落边界（`settings:` 并非从 `tabs` 一直延伸到文件尾）。
- e2e 服务器吃的是 `ui/dist` 内嵌产物，改完源码必须重跑构建（`just ui-e2e`
  自带 `pnpm build`，单独 `tsc` 不刷新 dist）。
