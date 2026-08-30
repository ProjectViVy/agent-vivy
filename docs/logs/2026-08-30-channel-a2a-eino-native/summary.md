# 2026-08-30 · 超级通道合同：Eino 原生 A2A 澄清

## 目标与背景

C0 合同把 A2A 写成后切 `plugins/a2a`、Task = Run。补一句产品问过的边界：Eino 原生支持 A2A，这批五个聊天插件会不会挡原生路线。

结论写进合同，不是新架构。

## 变更内容

- `docs/architecture/VIVY-CHANNEL-PACK.md` — §1 拍板、§5 采纳/拒绝、§14.1 源、§15.1 叠法、§18 / §19 / §20 C9 / §21。
- `docs/architecture/VIVY-PLUGIN-SPEC.md` — 禁止项：`eino-ext/a2a` 与 `RegisterServerHandlers`。
- `docs/research/README.md` — 14a 一行。
- `docs/TODO.md` §10 — 记一笔。

锁定：

- Eino 核心没有 A2A 线协议。进程内 `AgentAsTool` / DeepAgent 不是 A2A，本批不碰。
- `eino-ext/a2a` 拆两层：偷 `models` + `transport`；禁止 `RegisterServerHandlers(adk.Agent)` 当网关。
- 后切叠法：codec → `plugins/a2a` → ChannelHost → `Service.Run` → 已有 ADK Runner。
- 本批五个聊天插件零 Eino import。C1–C8 不改形状。

## 明确未做

- 未实现 A2A、未引入 `eino-ext/a2a` 依赖、未改内核 / SDK / UI。
- 未开 C9 能力提案。
