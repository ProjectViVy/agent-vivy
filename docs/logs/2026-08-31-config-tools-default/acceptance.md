# Acceptance — 2026-08-31 config tools 默认面保留

人怎么确认：

1. 本地 config.yaml 的 `tools:` 段只保留 `approval`（无 `enabled`）。
2. `just run` 正常启动，日志无
   `tools.enabled must list at least one tool`。
3. 设置 → 工具：卡片显示"配置默认值：26 个（当前未写覆盖层）"，
   绝大多数工具开关为激活态。
4. 显式想关闭全部工具的用户仍需在设置页写覆盖层（纯对话模式），
   而不是在 config.yaml 写空表——空表依旧被启动校验拒绝。
