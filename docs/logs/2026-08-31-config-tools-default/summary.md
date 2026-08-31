# 2026-08-31 — config.yaml 省略 tools.enabled 时保留代码默认工具面

## What changed

`internal/config` 的 `Tools.UnmarshalYAML` 无条件用文档里的 `enabled`
键覆盖接收者——而 `Load` 从 `Default()`（26 工具启用面）起步再叠加
YAML。于是任何带 `tools:` 段但省略 `enabled` 的 config.yaml 都会把
启用面清成空，随后被 Validate 以
`tools.enabled must list at least one tool` 拒绝，进程
`startup aborted`。

本次在解码时检测 `enabled` 键是否显式出现：省略 → 保留代码默认；
显式写空表 → 仍解码为 nil 并由 Validate 拒绝（行为不变）。

动机：两层工具落地后清理宿主本地 `config.yaml` 遗留的
`tools.enabled: [echo_info, write_note]` 两行（正是"模型只看见
echo_info/write_note/skill 三个工具"的直接原因），重启即触发上述
拦截，暴露此解码缺陷。

## What was explicitly not done

- 不改 config.example.yaml（仍显式列出全量默认，作为文档样例）。
- 不为 `network_search.provider` / `approval.expiration` 做同样的
  "省略保留默认"处理（它们的代码默认本就是零值，无实际差异）。

## 关联

- `docs/logs/2026-08-31-two-tier-tools/`（工具面产品语义）
- `docs/logs/2026-08-31-run-events-budget/`（同日 TT-4 熔断修复）
