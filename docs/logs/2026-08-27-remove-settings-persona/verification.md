# verification.md — 删除设置中的「人格」面板

命令均在仓库根目录 `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy` 执行（标注除外）。

## 自动化门禁

运行 `just ci`（fmt-check → vet → test → headless-compile → ui-ci: pnpm typecheck + vitest + vite build）：

- `gofmt -l`、`go vet ./...`、`go test ./...` 全绿。
- `pnpm typecheck`（tsc --noEmit）通过。
- `pnpm test`：15 个测试文件 / 105 个用例全部通过（含 `demo-api.test.ts`、`i18n/index.test.ts`）。
- `pnpm build`（vite build）成功，dist 产物生成。

结果：`just ci` exit code 0（跑过两轮：首次在并行 lane 落地前，第二轮在 `30c78b8` 之后、对当前 HEAD 复验，均绿）。

## 浏览器实走冒烟（split Vite :3015）

开发链路已在运行（`just run` + `cd ui; pnpm dev`），`http://127.0.0.1:3015` 返回 200。用 Playwright 脚本实走（脚本为临时文件，验证后已删除）：

```json
{
  "settingsHeading": 1,
  "tabs": ["通用", "模型", "工具", "Vivy 功能", "通道\n预览", "网络\n预览", "语言\n预览", "压缩\n预览", "自进化\n预览", "沙箱\n预览"],
  "personaTabGone": true,
  "personaConfigGone": true,
  "personaPageHeading": 1,
  "identityButton": 1
}
```

- 设置页 tab 列表不再包含「人格」，页面不再出现「人格配置」文本。
- 侧栏「人格」链接仍可达 `/persona` 页面（heading + IDENTITY.MD 按钮存在）。
- 控制台无相关报错。

## 未验证

- `pnpm e2e`（Playwright 全套）未运行；本次仅手工冒烟 `runtime.spec.ts` 中受影响的两步已从测试中删除，其余 e2e 用例语义不受影响。