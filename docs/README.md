# AGENT-VIVY 文档总索引

> 本目录是 Vivy 物种内核与第一方 Studio 的全部文档。
> 产品规则正本：`architecture/VIVY-STUDIO.md`。
> 更新：2026-08-25

## 1. 推荐阅读顺序

1. `TODO.md` §0 — 当前姿态与剩余工作（活看板）
2. `architecture/VIVY-STUDIO.md` — 产品契约正本
3. `IMPLEMENTATION-PLAN.md` — V0 架构落地翻译
4. `research/README.md` — 决策与设计档案索引（V0 前置档案，含自己的阅读顺序）

## 2. 顶层活跃文档（路径稳定，勿移动）

以下文件被仓库根 `README.md` 与 `internal/runtime/` 代码注释引用，保持顶层路径不变：

| 文件 | 用途 |
|---|---|
| `TODO.md` | 活看板：剩余工作见 §0.1，关闭轨迹归档在 `logs/` |
| `IMPLEMENTATION-PLAN.md` | V0 实现计划与架构翻译（TODO 的架构参考） |
| `AGENT-VIVY-ARCHITECTURE-V0.md` | ADR 基线（ADR-001..009+），记录 V0 形态 |
| `GOAL-AGENT-HARNESS-ROADMAP.md` | Harness 强化路线图（H0–H10） |
| `eino-capability-verify.md` | Eino v0.9.13 能力验证记录（A1，checkpoint-bridge GO） |
| `secret-redaction-audit.md` | 密钥脱敏审计（E3 / AS-9） |
| `v1-minimal-agent-proposal.md` | V1 最小 agent 层能力提案（MA-1..MA-4） |

## 3. 子目录

| 目录 | 内容 |
|---|---|
| `architecture/` | 产品契约正本：VIVY-STUDIO、SELF-EVOLVING-GATEWAY、VIVY-ASSEMBLY、VIVY-CHANNEL-PACK、VIVY-PLUGIN-SPEC、VIVY-WORLDVIEW、VIVY-GATEWAY-AND-STUDIO、ACP-REMOTE-CONTROL-PROPOSAL、hitl-review-center |
| `dev/` | 开发过程报告与实现记录：`NEW_UI_ARCHITECTURE.md`、`PHASE1..4` 报告、`real-provider-smoke.md`、`sandbox.md` + `SANDBOX-IMPLEMENTATION-SUMMARY.md`、`ui-migration/`（UI 迁移六篇，入口 `ui-migration/UI_MIGRATION_README.md`） |
| `research/` | 决策与设计档案（V0 前置）：方向、PRD、组装选项、参考索引、GO/NO-GO、开放项、DSH 差距、HITL UI 调研等；索引见 `research/README.md` |
| `logs/` | 按日期归档的验收/发布记录（`2026-08-12-hitl-release-closure`、`2026-08-16-studio-lifecycle`、`2026-08-25-todo-board-archive`） |

## 4. 维护规则

- 新增文档必须在本索引登记一行用途；过程性报告进 `dev/`，日期性验收进 `logs/<日期-主题>/`。
- 被代码注释或根 `README.md` 引用的文档不得移动路径。
- `research/` 目录遵循自己的维护规则（见 `research/README.md` §6）。
