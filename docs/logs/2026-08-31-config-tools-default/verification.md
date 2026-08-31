# Verification — 2026-08-31 config tools 默认面保留

## 命令与结果

```text
go test ./internal/config/ -count=1
  -> ok  agent-vivy/internal/config  1.150s

just ci
  -> EXIT=0（fmt/vet、Go 全量、ui 175 测试、ui build 全绿）
```

## 新增测试

`TestToolsSectionWithoutEnabledKeepsDefault`（internal/config/config_test.go）：

- `tools:` 段省略 `enabled` 键（其余字段保留）→ 加载成功，
  `cfg.Tools.Enabled` 与 `Default().Tools.Enabled` 等长（26）；
- 显式 `enabled: []` → Load 返回校验错误，不静默放行。

## 真路径证据（修复前）

删除本地 config.yaml 的 `enabled:` 两行后 `just run`：

```text
startup aborted: invalid config config.yaml:
  tools.enabled must list at least one tool
```

修复后同一份 config.yaml 启动成功（见本日志 acceptance 的复验步骤）。
