# 验证记录（2026-08-27，设置 → 语言）

## 执行的命令与结果

- 仓库根目录 `just ci`（fmt-check → vet → go test ./... → headless-compile →
  ui-ci[pnpm install --frozen-lockfile → typecheck → vitest → vite build]）：
  **通过，exit code 0**。UI 单测 105 passed（15 files），其中
  `diva-preview-data.test.ts`（更新后的语言排除断言，2 tests）、
  `i18n/index.test.ts`（9 tests）均通过；vite 生产构建成功（2201 modules）。
- 浏览器真实路径 `just ui-e2e`（pnpm build → playwright，webServer 自起
  `go run ./cmd/vivy`，E2E_ADDR 127.0.0.1:8799）：
  - **新增 `ui/e2e/language-setting.spec.ts` 通过（530ms）**：深链
    `/settings?tab=language` → 语言分区选中且中文文案可见 → 点击 English →
    立即切换为英文（`Pick the interface language…`、`Current language`）→
    `localStorage['vivy.language'] === 'en'`、`document.documentElement.lang === 'en'`
    → 刷新后保持 → 切回中文恢复。
  - 既有 `runtime.spec.ts` 与 `welcome-wizard.spec.ts` **失败，判定为过期规格、
    与本次改动无关**：两处断言文案（「密钥只由运行环境管理」、
    「API 密钥通过运行环境变量注入，不在界面中填写或保存。」）在全量
    `ui/src` 中已不存在（密钥提示已被 i18n 改写，现文案见 `ui/src/i18n/zh.ts`
    的 `catalogKeyHint` / `secretNote` 等键），且本次改动未触碰相关组件
    （`ModelSettingsCard`、`WelcomeWizard`）。已按 rulebook「todolist-capture-required」
    登记为 `docs/TODO.md` §0.1 的 `UI-E2E-STALE`，不在本迭代修复。

## 验证结论

- `just ci` 全绿；语言分区真实路径（切换 + 持久化 + 深链）经 Playwright 全通过。
- 未验证项：无。除语言分区外未做交互回归；两条过期 e2e 规格为既有问题，见 §0.1。