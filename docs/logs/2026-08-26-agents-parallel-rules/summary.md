# 2026-08-26 — AGENTS.md 并行 worktree 隔离与按交付提交规则

## What changed

`AGENTS.md`（仓库治理文档）新增两条规定与一节说明，来源是对 agent-diva
仓库规则（LOCK.md 互斥、原子 commit、worktree 隔离）的对比评估与用户决策：

1. **`parallel-worktree-isolation`（硬性要求）** + 新章节
   "Parallel lanes (worktree isolation, hard requirement)"：
   共享根工作树最多承载一个活跃写 lane。第二个并发 lane（另一 agent 会话、
   Studio 会话或人工改动），或根树不干净时启动的新功能，必须在独立
   `git worktree` + 分支上开发，经分支合并/PR 落回；禁止在另一 lane 活跃时
   编辑根树，禁止在根树堆叠无关主题。不引入 `LOCK.md` —— 用结构隔离取代
   协议协调。
2. **`commit-one-concern-per-deliverable`**：每个完成的交付在完成时提交为
   一个单一关注点的 commit，只 stage 本交付的明确路径，剔除临时产物，不卷入
   无关脏改；推送仍需用户明确授权。（agent-diva 的 per-update auto-commit
   的弱化移植：commit 跟随交付而非每次更新。）
3. "Not ported from agent-diva on purpose" 段落改写：`LOCK.md` 互斥、根
   `TODOLIST.md`、`/new-command`、per-update auto-commit、回复前缀仍不移植，
   并说明并发与提交卫生由上述两条原生规则替代。

## Scope

- `AGENTS.md`：新增章节 + Rulebook 两条 + not-ported 段改写。
- `docs/TODO.md` §0.1：新增 `PROC-COMMIT` 条目（见下）。
- 本迭代日志。

## Decision origin

用户在对比 agent-diva 规则后拍板：worktree 隔离为**硬性要求**（此前已因
多 lane 共用根树吃亏）；原子 commit 按建议移植弱化版；LOCK.md 锁机制
暂不引入。

## Explicitly not done

- 未引入 `LOCK.md` 锁文件（结构性隔离替代；若将来不够再评估）。
- 未引入 per-update 自动提交与 `[I strictly follow the rules]` 回复前缀。
- 根树现存的三个已完成但未提交主题（evolution 页 / welcome wizard /
  chat message actions）本次不拆分、不提交 —— 记入 `docs/TODO.md` §0.1
  `PROC-COMMIT`，留待按新规则拆分入库或人工确认后合并。
- 无任何内核 / UI 代码改动。
