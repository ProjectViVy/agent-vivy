# Acceptance — 2026-08-26 AGENTS.md 并行与提交规则

## 人工验收（产品/用户视角）

1. 打开仓库根 `AGENTS.md`：
   - "Validation" 一节之前可见新章节 **"Parallel lanes (worktree isolation,
     hard requirement)"**，写明根树单一活跃 lane、`git worktree add
     ../agent-vivy-<slug> -b feat/<slug>`、lane 经分支/PR 落回、无 LOCK.md。
   - Rulebook 末尾新增两条：`parallel-worktree-isolation`（标注 hard
     requirement）与 `commit-one-concern-per-deliverable`（推送仍需授权）。
   - "Not ported from agent-diva on purpose" 段已改写：说明锁文件被结构性
     worktree 隔离替代、提交跟随交付而非每次更新。
2. `docs/TODO.md` §0.1 出现 `PROC-COMMIT`（根树三个未提交主题待拆分入库）。
3. 行为验收（下次并行时生效）：再开一个 agent 会话/Studio 会话动这个仓库时，
   它应当自建 worktree + 分支工作，而不是编辑根树；本仓库后续每个完成的交付
   以独立单一关注点 commit 落库（本次交付本身就是第一个示范：仅含
   `AGENTS.md` 与本日志，未卷入根树其他脏改，未推送）。
