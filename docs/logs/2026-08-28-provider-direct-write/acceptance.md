# 验收 — 2026-08-28 provider-direct-write

## 用户可见：设置 → 模型

1. **后端持久化注册表**：新增自定义供应商（显示名 + bundle + Base URL +
   默认模型 + 模型列表 + API Key）→ 刷新页面条目仍在（不再 localStorage）；
   `data/agent-home/settings.yaml`（默认 `~/.vivy/settings.yaml`）可见
   `providers:` 段，含 `api_key`（0600）。
2. **密钥不回传**：`settings/providers` 返回每条 `api_key_set`，界面/网络面板/
   日志无密钥原文；`settings/update` 载荷不再携带 key。
3. **选模型即同步环境**：点击该供应商模型 → 进程内 `os.Getenv` 的
   `VIVY_API_BASE` 与活动束 `env_key` 已同步为新值（写时 `ApplySettingsEnv`）；
   重启后仍生效（启动 overlay 重放同文档）。
4. **编辑/删除**：编辑对话框改条目（id 不变）；删除条目 → 列表消失；若曾是
   active，文档清覆盖层，重启回落到运行束 env_key。
5. **冲突与校验**：同 (bundle,base_url) 重复注册被后端拒绝；坏 URL / 换行 key
   保存报错不落盘。

## 系统级用户工作空间

6. 在以临时 `HOME`/`USERPROFILE`（或 `VIVY_USER_HOME`）启动、且无
   `config.yaml` 的默认场景下首次启动 → 自动创建 `<home>\.vivy\workspace` 与
   `<home>\.vivy\settings.yaml`；提供 `config.yaml` 时路径仍走显式值。

## 如何验证

- 起 `just run` + `cd ui; pnpm dev`，开 `http://127.0.0.1:3015/settings?tab=model`
  按上文 1–6 逐项操作。
- 后端单测即含全部断言（见 verification.md），浏览器冒烟由用户在空闲端口代验。