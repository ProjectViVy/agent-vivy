# 验证 — 思考模式（UI-COMPOSER / UI-CHAT-TOOLBAR 可行部分）

日期：2026-09-02。全部命令在仓库根目录（`agent-vivy`）执行。

## 内核与 RPC 定向测试

```text
go test ./internal/provider -run 'Thinking' -count=1
  → ok  agent-vivy/internal/provider  0.385s
go test ./internal/runtime -run 'Thinking' -count=1
  → ok  agent-vivy/internal/runtime   2.031s
go test ./internal/runtime -run 'Thinking' -race -count=1
  → ok  agent-vivy/internal/runtime   2.936s
go test ./internal/rpc -run 'TestTurnStartThinkingRoute' -count=1
  → ok  agent-vivy/internal/rpc       1.070s
```

覆盖点：

- `TestResolvingModelInjectsClaudeThinking` / `TestResolvingModelWithToolsInjectsThinking` —
  本地 Anthropic 形状 httptest 服务上断言**出站请求体**出现
  `thinking: {"type":"enabled","budget_tokens":4096}`（绑定工具与裸调用两条路径）。
- `TestResolvingModelOmitsThinkingWithoutRequest` — auto / off / 未设置三种情况均
  不出现 `thinking` 键（与既有流量逐字节一致）。
- `TestResolvingModelThinkingGatedOnMetadata` — claude-3-5-sonnet 即便请求 on 也
  不注入（元数据门禁生效）。
- `TestNormalizeThinkingMode` / `TestRunWithOptionsRejectsInvalidThinkingMode` —
  `""→auto` 归一；`execute` 这类非法值在持久化前被拒（`ErrInvalidThinkingMode`）。
- `TestRunWithOptionsCarriesThinkingModeToTheModel` — 捕获型模型确认 run 作用域
  ctx 里的 `ThinkingModeFromContext` 全程读到 `on`（默认 run 读到 `auto`）。
- `TestTurnStartThinkingRoute`（rpc 集成）— `turn/start` 带非法 thinking →
  InvalidParams；带 `on` → 正常建 run；`session/context` 回报
  `thinking_supported` 字段。

## UI

```text
cd ui; pnpm typecheck
  → 通过（无类型错误）
pnpm test -- --run
  → 24 个测试文件 / 197 个测试全部通过
```

- `store.test.ts` 新增 'carries the thinking preference through the queue'：
  运行中入队的消息保留逐条 thinking 偏好，run 完成后 `startTurn` 收到
  `('s1','think hard','normal',undefined,undefined,'on')`。
- 既有队列测试更新为 6 参精确匹配（`vi.waitFor` 严格参数计数）。

## 产品门禁

```text
just ci
  → CI-EXIT:0（后台运行，日志尾部确认；含 gofmt/build/vet、全量 go test、
    ui build + vitest、headless 构建标签、plugin-ci 六插件）
```

## e2e 冒烟（split Vite 真实路径）

```text
just ui-e2e
  → 首轮：新 spec 的“思考模式按钮 count 0”断言通过，但“附件按钮”定位失败——
    附件控件是 <label aria-label="附件">（非 button role），属测试定位器写法
    错误而非产品缺陷；页面快照确认工具栏/输入框/发送键全部正常渲染。
  → 修正为 getByLabel('附件') + 文本框断言后复跑：thinking-gate.spec.ts 通过，
    全套 e2e 无回归（终轮结果见下）。
```

复跑结果：e2e 全套通过（18 passed + 新 spec，0 failed），`E2E-EXIT:0`。

## 明确不在本轮验证范围

- OpenAI 系 `reasoning_effort` 未接线（拍板时已明确不做），无从验证。
- 真实 Anthropic 上游的出站行为由本地 Anthropic 形状服务断言出站 JSON 承担；
  未消耗真实 API key。
