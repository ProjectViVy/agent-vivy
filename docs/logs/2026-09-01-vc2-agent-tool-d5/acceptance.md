# 验收：人如何确认它工作了

## 产品视角

1. **模型能看到 agent 工具**：启动 vivy（默认配置），任一会话里向模型
   询问可用工具（或打开 Settings 工具面），`agent` 出现在工具清单中，
   描述为"委托子任务给干净上下文的只读子代理"。
2. **一次真实委托**：配好 provider key 后，对 vivy 说
   "用 agent 工具调查 <某个读类问题>，然后把结论告诉我"。观察：
   - 会话出现一条 `agent` 工具调用记录，随后模型引用其结果作答；
   - Runs/子运行视图（或 Journal）出现一条 Kind=child、ParentID=当前
     run 的子运行，状态终态 completed；
   - Token 统计面板的会话/模型用量**包含**子代理消耗（成本汇总回父
     会话，D9 面板直接可见）。
3. **面具生效**：带 `mask` 参数（如 "terse reviewer"）委托时，子代理
   行为风格随提示变化（Journal 中子运行的首条 system 消息含
   `Persona hint (mask): …`）。
4. **审批并入父会话**：若子代理调用进入 ask 分级的效果性工具（在只读
   面内不会，此为防御语义验证），审批请求出现在父会话的 Review Center，
   Kind=child。
5. **不可嵌套**：要求子代理"再派生一个 agent"时，其工具面中没有
   `agent` 工具，子代理只能直接作答。
6. **无 MCP**：子代理工具面不含任何 `mcp_*` 工具。

## 无 key 环境的最小验证（本片实际执行的）

- `just ci` 全绿（见 verification.md）；
- worker 协议层系统消息贯穿测试证明子代理 harness 收到并使用了 persona
  提示；
- app 层守卫测试证明未装配/无 run/超并发时委托会清晰失败而不是悬挂。

## 回滚

单 commit（`feat/vc1a-bash-tool` 分支），revert 即整体退场；无 schema
迁移、无数据格式变更。
