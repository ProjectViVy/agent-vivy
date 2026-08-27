# 验证记录 — 2026-08-27 供应商目录折叠移植

## 自动化门禁

| 命令 | 结果 |
|---|---|
| `cd ui; pnpm typecheck` | ✅ 无错误 |
| `cd ui; pnpm test`（vitest） | ✅ 13 个文件 / 68 用例全过（含新增 `provider-catalog.test.ts` 13 用例；i18n zh/en 结构同步测试过） |
| `just ci`（仓库根，= fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]） | ✅ 全绿，ui build `✓ built in 3.20s`（仅既有 chunk>500kB 警告） |

## 浏览器冒烟（split pair：`just run` :8787 + `cd ui; pnpm dev` :3015）

在 `http://127.0.0.1:3015/settings?tab=model` 逐项验证（内置浏览器 +
DOM 快照断言）：

1. **目录渲染**：常用供应商 27 行平铺（OpenRouter…Mimo），当前配置
   （mock）命中的 Mock 行带「当前」徽标且选中；折叠行
   「更多供应商 20」存在（数量=20 与 diva 名单一致）。
2. **折叠展开**：点击折叠行后 CherryIN / Together AI / Yi (01.AI) /
   PPIO / Cerebras 等折叠供应商出现。
3. **搜索绕过折叠**：搜索 "yi" → 仅平铺 CherryIN 与 Yi (01.AI)（子串
   命中），折叠行隐藏，DeepSeek/Mock 等被过滤。
4. **选择供应商（关键映射）**：搜索 deepseek → 点击 DeepSeek 行 →
   右栏显示 DeepSeek / https://api.deepseek.com/v1 / bundle 标签
   `openai` 与 5 个模型；表单填充 Provider=openai、
   默认模型=deepseek-v4-pro、Base URL=https://api.deepseek.com/v1。
5. **选模型**：点击 deepseek-chat → 默认模型输入更新为 deepseek-chat。
6. **保存落盘**：点击「保存真实设置」→ 无错误；顶栏切换器变为
   "DeepSeek | deepseek-chat"；DeepSeek 行获得「当前」徽标；
   `data/settings.yaml` 实际写为
   `provider: openai / default_model: deepseek-chat / base_url: https://api.deepseek.com/v1`
   （UI→RPC→settings.Save 全链路实证）。
7. **顶栏切换器**：下拉显示「当前配置 DeepSeek | deepseek-chat」与
   「DeepSeek 可选模型」（deepseek-v4-pro / v4-flash / coder /
   reasoner，当前模型已去重）。
8. **环境恢复**：冒烟后把 `data/settings.yaml` 恢复为 mock 三元组并
   刷新页面，顶栏回到 "Mock | mock"、Mock 行重获「当前」徽标。

## 工具备注

冒烟所用内置浏览器（IAB）对滚动容器内元素的定位器点击会超时
（actionability 探测缺陷），改用 DOM 节点点击完成全部交互；属浏览器
工具层怪癖，非产品缺陷——元素对真实用户点击响应正常（键盘/坐标/节点
三种路径均触发过同一 React 处理器）。

## 结论

`just ci` 绿 + 真实路径冒烟通过，符合 `smoke-for-user-visible-change`
与 `just-ci-is-the-gate` 规则。
