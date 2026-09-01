# CMP-2 — 压缩摘要独立模型 `summary_model`

## Scope

- `internal/config`：`CompactionConfig.SummaryModel string \`yaml:"summary_model"\`` —
  同一活跃 provider 上的可选更便宜模型 id，替代主模型生成压缩摘要；空 = 沿用
  主模型。Validate 归一（TrimSpace，禁换行/NUL，与 `runtime.small_model` 同款
  结构校验；未知 id 不在启动期拒绝——调用失败即 failover）。
- `internal/provider`：导出 `NewOverrideModel(catalog, resolver, modelID)` ——
  D9 单数据源的辅助模型钉 id 构造器（modelOverrideSource + resolving model，
  调用时解析活跃 provider 的 live spec）；`TitleCandidates` 重构复用之。
- `internal/runtime`：`EngineConfig.SummaryModel`（D-007 层界：app 侧以别名
  `runtime.SummaryModel = model.BaseModel[*schema.Message]` 不透明传值，app 不
  import eino）。`buildCompactionHandlers` 接入 Eino summarization 原生
  Failover：设了覆盖模型时 primary = 覆盖模型，`Failover{MaxRetries:1,
  BackoffFunc:0, GetFailoverModel: 主模型}`；GetFailoverModel 重建默认输入形状
  —— 剥离原始前导 system 消息后夹回中间件的 system/user 摘要指令（不复制
  system）。未配置覆盖时不接 Failover（同一模型重试一次无意义，行为与旧版
  完全一致）。
- `internal/app`：启动与 settings-save 重载两条 `buildEngineConfig` 路径都传
  `summaryModel`（cfg 归一后的 id → `provider.NewOverrideModel`）。
- `config.example.yaml`：`compaction.summary_model` 注释示例。

## 语义

| 配置 | 摘要生成 | 失败行为 |
|---|---|---|
| `summary_model` 空 | 主模型（旧行为） | 无 Failover，与旧版一致 |
| `summary_model` 设了 | 覆盖模型 1 次 | 失败 → 主模型 fallback 恰 1 次（无退避），再失败 run 响亮报错 |

## Not done

- settings.yaml 覆盖层 / UI 面板未加 summary_model 字段（本行只要求配置面；
  overlay 走 settings RPC+UI 需另开切片）。
- reduction 层不受影响（确定性清屏不用模型）。
- 手动 CompactSession 的摘要模型选择未改（沿既有路径，超出本行口径）。
