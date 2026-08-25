# Verification — 2026-08-25 VIVY 界面 i18n 国际化支持

## `just ci`（仓库根目录）

结果：**通过**（exit 0）。全链路：

- gofmt（cmd / internal / sdk / ui 的 *.go）：无未格式化文件
- `go vet ./...`：通过
- `go test ./...`：全部 ok（含 `-tags vivy_headless` 的 headless-compile）
- UI `pnpm install --frozen-lockfile` + `pnpm typecheck`：通过
- UI `pnpm test`：**9 个测试文件 / 40 个测试全过**，含新增 `src/i18n/index.test.ts`（9 个用例：zh/en 键一致性、插值、兜底、持久化、DOM lang）以及此前因缺 `@` 别名而失败的 rpc / runtime-config / run-subscription / store / demo-api 测试
- UI `pnpm build`：vite build 成功（2188 modules）

> 说明：本轮中途发现 `vitest.config.ts` 缺少 `resolve.alias['@']`，导致 5 个 lib 测试文件 `Cannot find package '@/i18n'`。已补别名修复，非跳过验证。

## 浏览器冒烟（http://127.0.0.1:3015，split Vite）

以 DOM 快照逐视图巡检，确认迁移后无回归、无 raw key 泄漏（默认 zh）：

| 验证点 | 结果 |
|---|---|
| 应用挂载、默认中文渲染 | ✅ 导航/顶栏/设置/聊天均中文 |
| /masks 面具库（含 capabilities 列表） | ✅ 无泄漏，卡片、能力、详情均本地化 |
| /skills /memory /notebook /persona /dashboard /mcp /cron-tasks /approvals /lifecycle | ✅ 全部正常渲染，巡检无 `xxx.yyy` raw key 泄漏 |
| 设置页「语言 预览」标签（对方实现） | ✅ 存在；实测点击 English **不**改变全局文案（预览限定），与对方标注一致 |

## 未验证项

- **设置页全局语言切换的真实点击**：`LanguagePicker` 未挂载进 `SettingsView`（对方文件，本轮不碰），浏览器无真实入口可驱动全局切换。EN 渲染正确性由 `src/i18n/index.test.ts` 确定性覆盖（词典对齐 + 切换 + 持久化）。接线完成后的验收路径见 `acceptance.md`。
- 截图（视觉）未留存：本环境模型以 DOM 文本核验为准，与既有日志做法一致。
