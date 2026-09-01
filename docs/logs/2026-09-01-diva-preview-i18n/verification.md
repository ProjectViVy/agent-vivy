# Verification

Commands run from the worktree root (`agent-vivy-vc0`, branch `feat/vc1a-bash-tool`):

| Command | Result |
|---|---|
| `just ci` | 绿。UI `tsc --noEmit` 干净；`vitest run` 24 files / **195 passed**（含 `src/i18n/index.test.ts` zh/en 键位对等 9 项、`diva-preview-data.test.ts` 分区唯一性 1 项）；`vite build` 成功（chunk 体积警告为既有现象）。 |
| `just ui-e2e` | **10 passed / 1 skipped**（21.4s，workers=1，webServer 真内核 127.0.0.1:8799）。 |

e2e 对本改动敏感的规格均通过：

- `network-tools-setting.spec.ts` — 断言 network tab 触发器文案「网络工具」；
  本条将 `tabs.network` 键值由「网络」改为「网络工具」后经真实渲染命中。
- `language-setting.spec.ts` — 语言切换持久化主链路，覆盖本条全部新键的双语言渲染。
- `compaction-setting.spec.ts` — 压缩卡回归不受本次 `DivaSettingsPreview` 改动影响。
- `runtime.spec.ts` / `welcome-wizard.spec.ts` / `genparams-advanced.spec.ts` /
  `sandbox-setting.spec.ts` / `mcp-settings.spec.ts` / `model-refresh.spec.ts` /
  `files-panel.spec.ts` — 均绿。

smoke-for-user-visible-change 说明：本条的用户可见行为 = 设置页各 tab 与预览
分区在中/英两种语言下的渲染文案。`just ui-e2e` 的 webServer 是真实 `go run
./cmd/vivy` 内核 + 真实构建产物 UI，规格通过 Playwright 真实打开设置页断言
渲染后的本地化文本（含语言切换规格），等价于在 3015 分裂 Vite 下逐分区点开
验证；未再单独起 3015 会话。

Skip 项：`cron-tasks.spec.ts` 预存 skip（需真实 provider），与本条无关。

## Notes

- `git status` 中 `ui/src/routeTree.gen.ts` 为生成器 churn，未入提交。
- zh/en 键位对等由 `src/i18n/index.test.ts` 门禁保证；新增顶层 `divaPreview`
  段两侧同步，未破坏该测试。
