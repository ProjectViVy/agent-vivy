# 验证记录 — 2026-08-30 目录厂商 API Key 可填写

仓库根：`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`。

## 单元与构建（根目录 `just ci`，全绿 exit 0）

```
just ci
# fmt-check ✓  vet ✓  go test ./... ✓  headless-compile ✓
# ui-ci：pnpm install --frozen-lockfile ✓
#        pnpm typecheck（tsc --noEmit）✓
#        pnpm test → 21 files / 174 tests 全部通过
#          （custom-providers.test.ts 19 项，含本次新增
#           目录厂商密钥落地条 4 项覆盖）
#        pnpm build（vite build）✓
```

新增测试断言：
- `catalogOverlayId('deepseek')` → `catalog-deepseek`；`isCatalogOverlayEntry` 判定前缀；
- `allProviderEntries` 隐藏 `catalog-*` 落地条、保留普通自定义条目（克隆）；
- `providerEntryByEndpoint` 按 `(bundle, base_url)` 命中（与后端 `ActiveKey`
  同口径）；
- `customApiKeySetFor` 目录端点命中注册表密钥覆盖返回 true；无覆盖/未配密钥
  返回 false。

## 浏览器冒烟（真实路径，Dev split pair）

前置：仓库自带的 split pair 已在运行（Vite `127.0.0.1:3015` → 代理 `/rpc`
到 `127.0.0.1:8787` 的 Studio console 后端）。冒烟脚本为**只读**：不输入
密钥、不触发写，仅点击目录行断言输入框态。

```
$env:NODE_PATH = "<repo>\ui\node_modules\.pnpm\node_modules"
node .workspace/smoke/catalog-key-smoke.cjs
```

结果（Playwright headless chromium，`http://127.0.0.1:3015/settings?tab=model`）：

```json
{
  "catalog": { "disabled": false, "placeholder": "sk-…（留空=应用时清除已配置密钥）",
               "hint": "写入本机用户工作区（~/.vivy/settings.yaml）；值不会回传界面或写入日志。下一条消息即生效。" },
  "mock":    { "disabled": true,  "placeholder": "内置 Mock 供应商无需 API Key。", "hint": "内置 Mock 供应商无需 API Key。" },
  "consoleErrors": []
}
```

- 目录厂商（DeepSeek）行：输入框**可编辑**，占位符/提示为可填写文案；
- Mock 行：输入框**仍禁用**，提示为 Mock 专属文案；
- 页面无 console 错误。

未做 WebSocket 层写路径验证（避免向用户正在使用的 Studio console
settings.yaml 写入测试密钥）；写路径由 `custom-providers` 纯函数单测 +
`just ci` 覆盖，密钥安全边界与自定义供应商同路径（`settings/providers/upsert`，
0600 写-only，D-010）。

## 明确跳过

- `ui/e2e`：不跑（需要独立后端 fixture 启动，且本迭代为 UI 状态/文案变更，
  已由 vitest + 上述 DOM 冒烟覆盖）。