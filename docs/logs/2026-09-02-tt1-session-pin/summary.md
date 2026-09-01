# TT-1 会话级 pin（老表，零新表）

## What changed

- **存储**：`RunStore` 契约新增 `ListRunsBySession(ctx, sessionID)`（internal/storage/contracts.go）——查**既有 runs 表**，不建新表（拍板：老表）。sqlite/postgres 双后端实现（`listRunsWhere` 共用谓词助手，`ORDER BY created_at, id` 对齐既有惯例，全状态、按创建序）；一致性套件 CN-19 `runs listed by session`（创建序断言 + 全状态断言 + 未知 session 空集负例），套件守卫 18→19。
- **运行时**：`Service.sessionMounts`（internal/runtime/service.go）在 drive 装配点接手：run 行创建后按 sessionID 列出既往 run，逐个 Replay journal 收集 `tool.mounted` payload（复用 TT-1a `recoveredMounts` 的解析模式），按创建序累积成一个 `MountedTools` 作为新 run 的 seed（当前 run 跳过——其时尚未挂载任何工具）。**resume 路径零改动**（TT-2 捕获快照优先，`resumeRun` 仍走 `pendingRun.mounted`）。
- **best-effort 语义**：列举失败或单个 run 的 journal 回放失败 → warn + 用已收集的部分（或空）继续，绝不 fail run；损坏 payload 同 `recoveredMounts` 降级为 warn 跳过。
- **治理不变**：挂载准入闸门（tooladapter）与审批路径零改动——pin 只恢复"可调用性"，hidden 工具的调用照常受策略/审批管辖。
- **测试**（internal/runtime/toolmount_session_test.go）：
  - `TestServiceSessionPinRestoresSkillMountedTools`：run1 `skill_view` 挂 echo_info → 完成；同 session run2 直接调 echo_info 成功，且 run2 无任何 `skill_view` 调用（脚本即无）。
  - `TestServiceSessionPinDoesNotLeakAcrossSessions`：同存储、另一 session 的 run 调 echo_info 被准入闸门拒绝——引擎节点错误判停，run `failed`（负例语义比"拒绝回喂"更强）。
  - 判别性已验证：drive 里摘除 seeding（probe `(*tools.MountedTools)(nil)`）后 run2 无法完成，还原后两测绿。
- **顺带修复**：`TestVC1Walkthrough` 在 `-race` 下超时——共享 `waitForRunStatus` 5s 截止太紧（bash 步骤逐个 spawn 真进程，race 下总时长 >5s；DB 在测试失败清理时被关，run 稍后完成打 ERROR log）。改为本测专用 `waitForWalkthroughStatus`（60s 截止、20ms 轮询）。

## Explicitly not done

- 不做 session 维度挂载的"解除"或审计 UI：pin 是只增的会话内便利语义，`tool.mounted` 事件流（TT-3）已是审计事实源。
- 子代理（child run）的 drive 路径未接 pin：child 工具面收窄为只读子集，会话 pin 是否注入 child 属 VC-2 agent 工具语义的后续决策，本片不扩。

## Notes

- 性能：seed 成本 = 同 session run 数 × journal 回放。drive 在 run 协程内异步执行，不占 RPC 延迟；长会话下回放的是过滤到 `tool.mounted` 事件前的顺序扫描，会话内 run 数通常为几十量级，可接受；若将来成为热点，可在 journal 加类型索引（未做，避免超老表拍板范围）。
- child run 的 journal 也带同一 sessionID，其 `tool.mounted` 事件会被后续主 run 的 seed 收编——与"会话内挂载只增"语义一致。
