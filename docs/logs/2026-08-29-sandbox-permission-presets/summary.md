# 2026-08-29 — 沙箱真实接入与三段权限预设

## 目标

把设置页「沙箱」从迁移预览升级为真实配置，并把聊天区「谨慎 / 智能 / 信任」接到会话级沙箱模式 + 审批策略。EINO 文件系统 / 命令 / HTTP 后端按当轮会话策略执行。

## 变更

- 领域：`PermissionPreset`（cautious / smart / trusted / custom）映射到 `read_only+ask` / `workspace_write+ask` / `danger_full_access+auto`。
- 存储：`SessionStore.UpdateSandboxPolicy`；创建会话写入默认预设。
- 运行时：`SandboxManager` 按调用带模式；`toolAdapter` 接入 `EvaluateApprovalPolicy`；`Service` 在 `turn/start` 钉死当轮 knobs。
- RPC：`session/set_permission`；`sessionResult` 回传沙箱字段；`settings/get|update` 增加 sandbox 分区。
- UI：`SandboxSettingsCard` 替换预览；聊天区权限选择器写当前会话；切到「信任」需确认。

## 明确未做

- OS 级进程沙箱（bwrap / Seatbelt / Windows ACL）
- deny glob、可编辑 `auto_approve_tools`、沙箱内工具超时
- 进行中 run 热切预设
- `danger_full_access` 仍不能逃出 per-run 工作区

未做项记入 `docs/TODO.md` §0.1：`SBX-OS` / `SBX-GLOB` / `SBX-LIVE`。
