# Verification — 2026-08-31 运行事件预算豁免

## 命令与结果

```text
go test ./internal/runtime/ -run 'TestReserveMappedBudget|Budget' -count=1
  -> ok  agent-vivy/internal/runtime  1.283s

just ci
  -> EXIT=0（fmt/vet、Go 全量测试、ui 175 测试、ui build 全绿）
```

## 新增测试

`TestReserveMappedBudgetSkipsStreamingDeltas`（internal/runtime/service_test.go）：

- 600 条 `model.delta` 映射事件在 MaxEvents=5 的账本上不产生预算错误
  （流式 chunk 不再消耗事件预算）；
- 语义事件（`model.completed` / `model.usage`）照常计费；
- 超过 MaxEvents 的语义事件批次仍以 `ErrBudgetExceeded` 熔断。

## 真路径证据（修复前的问题）

- `data/logs/vivy.log.2026-08-31`：14:36:36 与 14:43:35 两条
  `run budget circuit breaker opened kind=events limit=512`（两次冒烟 run）。
- 设置→工具 卡片保存链路真路径验证：勾选 list_dir/read_file →
  `tools/set-active` → `data/settings.yaml` 出现 `tools_enabled`
  四项 → 卡片回显"当前使用自定义覆盖层"。链路可用，随后用
  「恢复配置默认」清空，回归 config 默认语义。

## 待人工复验

用户重启 `just run` 后：中文请求"你现在有什么工具？能看到工作区里
有什么吗？"应在 512 预算内完整跑完（多轮工具调用 + 流式回复），
不再出现 safety budget 中断。
