# 分支回收与主线合并

## 变更

- 将 `feat/chatbox-buttons` 合并到 `main`，保留聊天框会话操作与执行模式改动。
- 将 `feat/skill-marketplace` 合并到 `main`，保留 Skills 后端目录/Marketplace RPC 与 UI。
- 将 `feat/cron-closed-loop` 合并到 `main`，保留定时任务调度、存储、RPC 与 UI。
- 将 `feat/channel-super-contract` 合并到 `main`，保留其通道计划文档备注。
- 合并冲突处保留两侧能力：动态 RPC capability 同时包含 Channel 与 Skills/Marketplace；SQLite migration016 同时包含消息出处列和 Cron 表；Postgres v14 原地升级同时补齐两类结构。

## 明确未做

- 未合并 `wip/pre-submodule-root-20260829`。它是旧的停放快照，包含大量过期/Studio 打包差异，不具备直接回收条件。
- 未删除任何分支、未推送远端。
- 根目录原有未提交的运行时测试、Studio 子模块、failure 文件和 Studio 日志保持不变。
