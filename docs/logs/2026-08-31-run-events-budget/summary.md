# 2026-08-31 — 运行事件预算对流式 delta 的豁免（TT-4）

## What changed

两层工具落地后的真路径冒烟暴露：带工具的中文请求连续触发
`run budget circuit breaker kind=events limit=512`。原因是
`internal/runtime/mapper.go` 对每个流式 chunk 落一条 `model.delta`
（思考模式另有 `model.reasoning_delta`），而
`internal/runtime/service.go` 的 `reserveMappedBudget` 对每个映射事件
计 1 个预算事件——任何正常长度的回复（数百 chunk）单独就能耗尽
MaxEvents=512，多轮工具调用必然中途熔断，表现为
"这次对话没有完成…reached a safety budget"。

修复：`reserveMappedBudget` 跳过流式 chunk 事件
（`EventModelDelta` / `EventModelReasoningDelta`）的事件计费。
失控防护不受影响——`model_calls`（每代生成 1 次计费，上限 32）与
`tool_calls`（上限 64）仍是真正的失控闸门；delta 载荷仍由 mapper
钳制在 max_event_payload_bytes 内。

同日清理：宿主根目录 `config.yaml`（gitignored 本地文件）遗留的
`tools.enabled: [echo_info, write_note]` 两行删除，恢复代码默认的
26 工具启用面——这正是"模型只报 echo_info/write_note/skill 三个工具"
的直接原因。settings.yaml 的 `tools_enabled` 覆盖层已通过设置页
「恢复配置默认」清空（该覆盖层此前用于验证保存链路，链路确认可用）。

## What was explicitly not done

- 不改 delta 的 journal 持久化契约（每 chunk 仍是持久事件，供 feed
  回放）；只豁免预算计费。
- 不调 MaxEvents 数值——512 对语义事件足够，调参是掩盖。
- 不处理空内容 chunk 仍落空 delta 事件的问题（journal 噪音，非本主题）。

## 关联

- `docs/TODO.md` §0.1 TT-4（本迭代关闭）
- `docs/logs/2026-08-31-two-tier-tools/`（暴露本问题的前置迭代）
