# 验收（2026-08-27，设置 → 网络工具）

人站在产品视角如何确认这次改动生效：

## 后端

1. `config.example.yaml` 的 `tools:` 下有 `network_search.provider` 注释段
   （说明 bing/google/searxng 需要环境变量、duckduckgo/wikipedia 免密钥，
   不写任何密钥值）。
2. 用 `just run` 启动后，调用真实 RPC（或在浏览器设置页查看）：
   - `settings/get` 返回 `network_search: { provider, config_provider, providers:
     [bing, google, duckduckgo, searxng, wikipedia] }`；无环境变量时
     `duckduckgo`、`wikipedia` 为 `configured: true, keyless: true`，
     bing/google/searxng 为 `configured: false` 并带 `env_key` 名。
   - `settings/update` 传 `network_search.provider: "wikipedia"` 后，
     `data/agent-home/settings.yaml` 出现 `network_search: provider: wikipedia`，
     `settings/get` 回显 `provider: "wikipedia"`；传未知 provider（如
     `yandex`）被拒绝（InvalidParams）。
3. 重启后（或直接看 `Search` 行为）：请求不指定 provider 时，首选
   `wikipedia` 生效；若首选缺少密钥，自动降级到免密钥 provider，搜索不失败。
4. 设置文档里已有 api_key 覆盖层时，保存网络工具设置**不会**清掉 api_key
   （卡片保存合并现有字段 + 自定义注册表回显）。

## 前端

1. `http://127.0.0.1:3015/settings?tab=network` 深链落在「网络工具」分区，
   无「预览」徽标；卡片标题与描述为真实文案（zh/en 跟随全局语言）。
2. 卡片显示真实 provider 名单：DuckDuckGo、Wikipedia 为「已配置/免密钥」；
   Bing、Google、SearXNG 在无环境变量时为「待配置」并显示
   「需要环境变量 BING_SEARCH_API_KEY」等提示（只显示变量名）。
3. 选择 Wikipedia → 点保存 → 出现「已保存，下次启动生效」→ 刷新后选择仍是
   Wikipedia（持久化）；恢复「自动」并保存后回到空首选。
4. `设置 → 网络`（旧预览 tab）不再存在；Agent-Diva 的 bocha/brave/zhipu
   假数据无处可见。

## 未验证 / 边界

- 未做真实网络搜索的线上调用验证（用户明确「不用端到端可用」）；provider
  请求路径由现有 `network_search_test.go` 的 httptest 覆盖。
- read_only 部署（SettingsPath 空）：卡片只读，保存禁用。
- 密钥值永不进入 UI、日志、设置文档或 RPC 响应（D-010）。